/*
Copyright 2026 The Ceph-CSI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rbd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	librbd "github.com/ceph/go-ceph/rbd"

	rbderrors "github.com/ceph/ceph-csi/internal/rbd/errors"
	"github.com/ceph/ceph-csi/internal/util/log"
)

const (
	// diffIterateFeatures are the image features that every image in a clone
	// chain needs for DiffIterate to use the fast-diff code path.
	diffIterateFeatures = librbd.FeatureExclusiveLock | librbd.FeatureObjectMap | librbd.FeatureFastDiff

	// invalidDiffIterateFlags are the image flags that force DiffIterate to
	// fall back to the slow code path.
	invalidDiffIterateFlags = librbd.FlagObjectMapInvalid | librbd.FlagFastDiffInvalid
)

// HasDiffIterateFeatures returns true when the comma separated list of image
// features (as used by the imageFeatures StorageClass parameter) contains all
// the features that are required for optimized DiffIterate operations.
func HasDiffIterateFeatures(imageFeatures string) bool {
	if imageFeatures == "" {
		return false
	}

	names := strings.Split(imageFeatures, ",")
	for i := range names {
		names[i] = strings.TrimSpace(names[i])
	}

	return uint64(librbd.FeatureSetFromNames(names))&diffIterateFeatures == diffIterateFeatures
}

// featureNames returns the names of the image features in the bitmask.
func featureNames(features uint64) []string {
	fs := librbd.FeatureSet(features)

	return fs.Names()
}

// imageChainLink is an image, or a snapshot of an image, that is part of the
// clone chain of a volume: either the HEAD of the volume image, or the parent
// snapshot that the volume image was cloned from.
type imageChainLink struct {
	pool      string
	namespace string
	imageName string
	imageID   string
	inTrash   bool

	isSnap   bool
	snapID   uint64
	snapName string
}

func (l *imageChainLink) String() string {
	spec := l.pool
	if l.namespace != "" {
		spec += "/" + l.namespace
	}
	spec += "/" + l.imageName
	if l.imageName == "" {
		spec += "<id:" + l.imageID + ">"
	}
	if l.isSnap {
		spec += fmt.Sprintf("@%s<id:%d>", l.snapName, l.snapID)
	}

	return spec
}

// openChainLink opens the image of the link by its ID. When atSnap is set and
// the link refers to a snapshot, the snapshot is selected on the opened image.
// The returned function must be called to release the image.
func (ri *rbdImage) openChainLink(link *imageChainLink, readOnly, atSnap bool) (*librbd.Image, func(), error) {
	if ri.conn == nil {
		return nil, nil, fmt.Errorf("can not open %s with unconnected image %q", link, ri)
	}

	ioctx, err := ri.conn.GetIoctx(link.pool)
	if err != nil {
		return nil, nil, err
	}
	ioctx.SetNamespace(link.namespace)

	var img *librbd.Image
	if readOnly {
		img, err = librbd.OpenImageByIdReadOnly(ioctx, link.imageID, librbd.NoSnapshot)
	} else {
		img, err = librbd.OpenImageById(ioctx, link.imageID, librbd.NoSnapshot)
	}
	if err != nil {
		ioctx.Destroy()
		if errors.Is(err, librbd.ErrNotFound) {
			err = fmt.Errorf("Failed as %w (internal %w)", rbderrors.ErrImageNotFound, err)
		}

		return nil, nil, fmt.Errorf("failed to open %s: %w", link, err)
	}

	release := func() {
		cErr := img.Close()
		if cErr != nil {
			log.WarningLogMsg("resource leak, failed to close %s: %v", link, cErr)
		}
		ioctx.Destroy()
	}

	if atSnap && link.isSnap {
		err = img.SetSnapByID(link.snapID)
		if err != nil {
			release()

			return nil, nil, fmt.Errorf("failed to set snapshot on %s: %w", link, err)
		}
	}

	return img, release, nil
}

// getChainLinkParent returns the parent snapshot of the link, or nil when the
// link does not have a parent.
func (ri *rbdImage) getChainLinkParent(link *imageChainLink) (*imageChainLink, error) {
	img, release, err := ri.openChainLink(link, true, true)
	if err != nil {
		return nil, err
	}
	defer release()

	parentInfo, err := img.GetParent()
	if err != nil {
		if errors.Is(err, librbd.ErrNotFound) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get parent of %s: %w", link, err)
	}

	return &imageChainLink{
		pool:      parentInfo.Image.PoolName,
		namespace: parentInfo.Image.PoolNamespace,
		imageName: parentInfo.Image.ImageName,
		imageID:   parentInfo.Image.ImageID,
		inTrash:   parentInfo.Image.Trash,
		isSnap:    true,
		snapID:    parentInfo.Snap.ID,
		snapName:  parentInfo.Snap.SnapName,
	}, nil
}

// hasValidObjectMap checks that the object map and fast-diff state of the
// link are valid.
func (ri *rbdImage) hasValidObjectMap(ctx context.Context, link *imageChainLink) (bool, error) {
	img, release, err := ri.openChainLink(link, true, true)
	if err != nil {
		return false, err
	}
	defer release()

	flags, err := img.GetFlags()
	if err != nil {
		return false, fmt.Errorf("failed to get flags of %s: %w", link, err)
	}
	if flags&invalidDiffIterateFlags != 0 {
		log.DebugLog(ctx, "%s has invalid object map or fast-diff (flags 0x%x)", link, flags)

		return false, nil
	}

	return true, nil
}

// enableDiffIterateFeatures enables the image features required for
// optimized DiffIterate operations on the image of the link, preserving all
// features that are already enabled. It returns true when features have been
// enabled, in which case the object maps of the image need to be rebuilt.
func (ri *rbdImage) enableDiffIterateFeatures(ctx context.Context, link *imageChainLink) (bool, error) {
	img, release, err := ri.openChainLink(link, false, false)
	if err != nil {
		return false, err
	}
	defer release()

	features, err := img.GetFeatures()
	if err != nil {
		return false, fmt.Errorf("failed to get features of %s: %w", link, err)
	}

	missing := diffIterateFeatures &^ features
	if missing == 0 {
		return false, nil
	}

	log.UsefulLog(ctx, "enabling image features %v on %s", featureNames(missing), link)
	err = img.UpdateFeatures(missing, true)
	if err != nil {
		return false, fmt.Errorf("failed to enable image features %v on %s: %w",
			featureNames(missing), link, err)
	}

	return true, nil
}

// rebuildObjectMap rebuilds the object map of the link. For a snapshot link
// the object map of the snapshot is rebuilt, otherwise the object map of the
// image HEAD.
func (ri *rbdImage) rebuildObjectMap(ctx context.Context, link *imageChainLink) error {
	img, release, err := ri.openChainLink(link, false, true)
	if err != nil {
		return err
	}
	defer release()

	log.UsefulLog(ctx, "rebuilding object map of %s", link)
	start := time.Now()
	err = img.RebuildObjectMapWithProgress(func(_, _ uint64, _ any) int {
		return 0
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to rebuild object map of %s: %w", link, err)
	}
	log.UsefulLog(ctx, "rebuilt object map of %s in %v", link, time.Since(start))

	return nil
}

// imageHead returns the link of the HEAD of the image of the link.
func imageHead(link *imageChainLink) *imageChainLink {
	head := *link
	head.isSnap = false
	head.snapID = 0
	head.snapName = ""

	return &head
}

// objectMapLinks returns the links whose object maps need to be valid for
// optimized DiffIterate operations on the child: the child itself, and when
// it has a parent, the parent snapshot and the HEAD of the parent image.
func objectMapLinks(child, parent *imageChainLink) []*imageChainLink {
	if parent == nil {
		return []*imageChainLink{child}
	}

	return []*imageChainLink{parent, imageHead(parent), child}
}

// reconcileChainLinkFeatures enables the missing DiffIterate features on the
// image of the link. When features were enabled, the object maps of the
// image HEAD and, for a snapshot link, of the snapshot are rebuilt, as
// enabling object-map invalidates all of them. Otherwise, only the object
// maps that are invalid are rebuilt.
func (ri *rbdImage) reconcileChainLinkFeatures(ctx context.Context, link *imageChainLink) error {
	enabled, err := ri.enableDiffIterateFeatures(ctx, link)
	if err != nil {
		return err
	}

	links := []*imageChainLink{link}
	if link.isSnap {
		links = append(links, imageHead(link))
	}

	return ri.rebuildObjectMaps(ctx, links, enabled)
}

// rebuildObjectMaps rebuilds the object maps of the links. Unless force is
// set, only invalid object maps are rebuilt.
func (ri *rbdImage) rebuildObjectMaps(ctx context.Context, links []*imageChainLink, force bool) error {
	for _, link := range links {
		if !force {
			valid, err := ri.hasValidObjectMap(ctx, link)
			if err != nil {
				return err
			}
			if valid {
				continue
			}
		}

		err := ri.rebuildObjectMap(ctx, link)
		if err != nil {
			return err
		}
	}

	return nil
}

// hasValidObjectMaps checks that the object maps of the volume image, and of
// its parent snapshot and parent image HEAD (if any) are valid.
func (ri *rbdImage) hasValidObjectMaps(ctx context.Context, child, parent *imageChainLink) (bool, error) {
	for _, link := range objectMapLinks(child, parent) {
		valid, err := ri.hasValidObjectMap(ctx, link)
		if err != nil || !valid {
			return false, err
		}
	}

	return true, nil
}

// flattenChainLink flattens the image of the link so that it no longer
// depends on its parent. ErrFlattenInProgress is returned when the flatten
// has been scheduled as a Ceph manager task.
func (ri *rbdImage) flattenChainLink(ctx context.Context, link *imageChainLink) error {
	if !link.inTrash && link.imageName != "" {
		parent := &rbdImage{
			ClusterID:      ri.ClusterID,
			Monitors:       ri.Monitors,
			Pool:           link.pool,
			RadosNamespace: link.namespace,
			RbdImageName:   link.imageName,
			ImageID:        link.imageID,
			conn:           ri.conn.Copy(),
		}
		defer parent.Destroy(ctx)

		return parent.flattenRbdImage(ctx, true, 0, 0)
	}

	// images in the trash can not be addressed by name for a Ceph manager
	// task, flatten them directly.
	img, release, err := ri.openChainLink(link, false, false)
	if err != nil {
		return err
	}
	defer release()

	_, err = img.GetParent()
	if errors.Is(err, librbd.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get parent of %s: %w", link, err)
	}

	log.UsefulLog(ctx, "flattening %s", link)
	err = img.Flatten()
	if err != nil {
		return fmt.Errorf("failed to flatten %s: %w", link, err)
	}

	return nil
}

// ReconcileDiffIterateFeatures makes sure that the volume image and its
// immediate parent are ready for optimized DiffIterate operations. The
// exclusive-lock, object-map and fast-diff image features are enabled where
// missing, and the object maps are rebuilt. Existing image features and data
// are preserved.
//
// The following steps are executed:
//  1. when the immediate parent image lacks the features, enable them and
//     rebuild the object maps of the parent snapshot and the parent image
//     HEAD. Invalid object maps of the parent are rebuilt as well.
//  2. when the volume image lacks the features, enable them and rebuild its
//     object map.
//  3. when the object map is not valid, rebuild the object map of the volume
//     image and check again.
//  4. when the object map is still not valid, flatten the parent image and
//     rebuild the object maps.
//
// rbderrors.ErrFlattenInProgress is returned when a flatten has been
// scheduled, and rbderrors.ErrDiffIterateNotReady when the object maps are
// still not valid; the caller is expected to retry in both cases.
func (rv *rbdVolume) ReconcileDiffIterateFeatures(ctx context.Context) error {
	err := rv.getImageID()
	if err != nil {
		return fmt.Errorf("failed to get image ID of %s: %w", rv, err)
	}

	child := &imageChainLink{
		pool:      rv.Pool,
		namespace: rv.RadosNamespace,
		imageName: rv.RbdImageName,
		imageID:   rv.ImageID,
	}
	parent, err := rv.getChainLinkParent(child)
	if err != nil {
		return err
	}

	// step 1: features of the immediate parent.
	if parent != nil {
		err = rv.reconcileChainLinkFeatures(ctx, parent)
		if err != nil {
			return err
		}
	}

	// step 2: features of the volume image.
	err = rv.reconcileChainLinkFeatures(ctx, child)
	if err != nil {
		return err
	}

	// step 3: rebuild the object map of the volume image when it is invalid.
	valid, err := rv.hasValidObjectMaps(ctx, child, parent)
	if err != nil || valid {
		return err
	}
	err = rv.rebuildObjectMap(ctx, child)
	if err != nil {
		return err
	}
	valid, err = rv.hasValidObjectMaps(ctx, child, parent)
	if err != nil || valid {
		return err
	}

	// step 4: flatten the parent image and rebuild the object maps.
	if parent != nil {
		err = rv.flattenChainLink(ctx, parent)
		if err != nil {
			return err
		}
	}
	err = rv.rebuildObjectMaps(ctx, objectMapLinks(child, parent), true)
	if err != nil {
		return err
	}
	valid, err = rv.hasValidObjectMaps(ctx, child, parent)
	if err != nil || valid {
		return err
	}

	return fmt.Errorf("%w: %s", rbderrors.ErrDiffIterateNotReady, rv)
}
