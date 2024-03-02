/*
Copyright 2020 The Ceph-CSI Authors.

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
	"strconv"

	"github.com/ceph/ceph-csi/internal/journal"
	"github.com/ceph/ceph-csi/internal/util"
	"github.com/ceph/ceph-csi/internal/util/log"
)

func createRBDClone(
	ctx context.Context,
	parentVol, cloneRbdVol *rbdVolume,
	snap *rbdSnapshot,
) error {
	// create snapshot
	err := parentVol.createSnapshot(ctx, snap)
	if err != nil {
		log.ErrorLog(ctx, "failed to create snapshot %s: %v", snap, err)

		return err
	}

	snap.RbdImageName = parentVol.RbdImageName
	// create clone image and delete snapshot
	err = cloneRbdVol.cloneRbdImageFromSnapshot(ctx, snap, parentVol)
	if err != nil {
		log.ErrorLog(
			ctx,
			"failed to clone rbd image %s from snapshot %s: %v",
			cloneRbdVol.RbdImageName,
			snap.RbdSnapName,
			err)
		err = fmt.Errorf(
			"failed to clone rbd image %s from snapshot %s: %w",
			cloneRbdVol.RbdImageName,
			snap.RbdSnapName,
			err)
	}
	errSnap := parentVol.deleteSnapshot(ctx, snap)
	if errSnap != nil {
		log.ErrorLog(ctx, "failed to delete snapshot: %v", errSnap)
		delErr := cloneRbdVol.deleteImage(ctx)
		if delErr != nil {
			log.ErrorLog(ctx, "failed to delete rbd image: %s with error: %v", cloneRbdVol, delErr)
		}

		return err
	}

	return nil
}

// cleanUpSnapshot removes the RBD-snapshot (rbdSnap) from the RBD-image
// (parentVol) and deletes the RBD-image rbdVol.
func cleanUpSnapshot(
	ctx context.Context,
	parentVol *rbdVolume,
	rbdSnap *rbdSnapshot,
	rbdVol *rbdVolume,
) error {
	err := parentVol.deleteSnapshot(ctx, rbdSnap)
	if err != nil {
		if !errors.Is(err, ErrSnapNotFound) {
			log.ErrorLog(ctx, "failed to delete snapshot %q: %v", rbdSnap, err)

			return err
		}
	}

	if rbdVol != nil {
		err := rbdVol.deleteImage(ctx)
		if err != nil {
			if !errors.Is(err, ErrImageNotFound) {
				log.ErrorLog(ctx, "failed to delete rbd image %q with error: %v", rbdVol, err)

				return err
			}
		}
	}

	return nil
}

func generateVolFromSnap(rbdSnap *rbdSnapshot) *rbdVolume {
	vol := new(rbdVolume)
	vol.ClusterID = rbdSnap.ClusterID
	vol.VolID = rbdSnap.VolID
	vol.Monitors = rbdSnap.Monitors
	vol.Pool = rbdSnap.Pool
	vol.JournalPool = rbdSnap.JournalPool
	vol.RadosNamespace = rbdSnap.RadosNamespace
	vol.RbdImageName = rbdSnap.RbdSnapName
	vol.ImageID = rbdSnap.ImageID
	// copyEncryptionConfig cannot be used here because the volume and the
	// snapshot will have the same volumeID which cases the panic in
	// copyEncryptionConfig function.
	vol.blockEncryption = rbdSnap.blockEncryption
	vol.fileEncryption = rbdSnap.fileEncryption

	return vol
}

func undoSnapshotCloning(
	ctx context.Context,
	parentVol *rbdVolume,
	rbdSnap *rbdSnapshot,
	cloneVol *rbdVolume,
	cr *util.Credentials,
) error {
	err := cleanUpSnapshot(ctx, parentVol, rbdSnap, cloneVol)
	if err != nil {
		log.ErrorLog(ctx, "failed to clean up  %s or %s: %v", cloneVol, rbdSnap, err)

		return err
	}
	err = undoSnapReservation(ctx, rbdSnap, cr)

	return err
}

func RegenerateSnapJournal(
	snapVolumeID,
	clusterName,
	owner,
	requestName string,
	cr *util.Credentials,
) (string, error) {
	ctx := context.Background()
	var (
		vi     util.CSIIdentifier
		rbdVol *rbdVolume
		err    error
		// ok     bool
	)

	rbdVol = &rbdVolume{}
	rbdVol.VolID = snapVolumeID
	rbdVol.ClusterName = clusterName
	rbdVol.Owner = owner

	// TODO: need to figure out how to get the snap parent name
	// For now, for testing using the volumeHandle to get UUID and then GetNameForUUID
	// to get the parent image name
	vi = util.CSIIdentifier{}
	err = vi.DecomposeCSIID(rbdVol.VolID)
	if err != nil {
		return "", fmt.Errorf("%w: error decoding volume ID (%w) (%s)", ErrInvalidVolID, err, rbdVol.VolID)
	}

	// TODO: find a way to get kms config

	rbdVol.Monitors, rbdVol.ClusterID, err = util.FetchMappedClusterIDAndMons(ctx, vi.ClusterID)
	if err != nil {
		return "", err
	}

	mappedPoolID, err := util.FetchMappedRBDPoolID(ctx, rbdVol.ClusterID, strconv.FormatInt(vi.LocationID, 10))
	if err != nil {
		return "", err
	}
	pID, err := strconv.ParseInt(mappedPoolID, 10, 64)
	if err != nil {
		return "", err
	}
	
	// TODO: get Pool
	poolName, err := util.GetPoolName(rbdVol.Monitors, cr, pID)
	if err != nil {
		return "", err
	}
	rbdVol.Pool = poolName

	// TODO: get journal pool
	rbdVol.JournalPool = rbdVol.Pool

	log.DebugLog(ctx, "monitors: %v, clusterID: %s, pool: %s, journalPool: %s", rbdVol.Monitors, rbdVol.ClusterID, rbdVol.Pool, rbdVol.JournalPool)

	err = rbdVol.Connect(cr)
	if err != nil {
		return "", err
	}

	snapJournal = journal.NewCSISnapshotJournal(CSIInstanceID)
	s, err := snapJournal.Connect(rbdVol.Monitors, rbdVol.RadosNamespace, cr)
	if err != nil {
		return "", err
	}
	defer s.Destroy()

	journalPoolID, imagePoolID, err := util.GetPoolIDs(ctx, rbdVol.Monitors, rbdVol.JournalPool, rbdVol.Pool, cr)
	if err != nil {
		return "", err
	}

	rbdVol.RequestName = requestName
	// TODO: currently there is no field for name prefix in VolumeSnapshotContent
	// one way of doing will be fetch the name prefix from its class name
	rbdVol.NamePrefix = "csi-snap-"

	imageData, err := s.CheckReservation(ctx, rbdVol.JournalPool, rbdVol.RequestName, rbdVol.NamePrefix, rbdVol.ParentName, "", util.EncryptionTypeNone)
	if err != nil {
		return "", err
	}

	if imageData != nil {
		rbdVol.ReservedID = imageData.ImageUUID
		rbdVol.ImageID = imageData.ImageAttributes.ImageID
		rbdVol.Owner = imageData.ImageAttributes.Owner
		rbdVol.RbdImageName = imageData.ImageAttributes.ImageName
		if rbdVol.ImageID == "" {
			err = rbdVol.storeImageID(ctx, s)
			if err != nil {
				return "", err
			}
		}

		if rbdVol.Owner != owner {
			err = s.ResetVolumeOwner(ctx, rbdVol.JournalPool, rbdVol.ReservedID, owner)
			if err != nil {
				return "", err
			}
		}
		// TODO: Update Metadata

		// As the omap already exists for this image ID return nil.
		rbdVol.VolID, err = util.GenerateVolID(ctx, rbdVol.Monitors, cr, imagePoolID, rbdVol.Pool,
			rbdVol.ClusterID, rbdVol.ReservedID, volIDVersion)
		if err != nil {
			return "", err
		}

		return rbdVol.VolID, nil

	}

	rbdVol.ReservedID, rbdVol.RbdImageName, err = s.ReserveName(
		ctx, rbdVol.JournalPool, journalPoolID, rbdVol.Pool, imagePoolID,
		rbdVol.RequestName, rbdVol.NamePrefix, rbdVol.ParentName, "", vi.ObjectUUID, rbdVol.Owner, "", util.EncryptionTypeNone,
	)
	if err != nil {
		return "", err
	}
	log.DebugLog(ctx, "reservedID: %s, rbdImageName: %s", rbdVol.ReservedID, rbdVol.RbdImageName)

	defer func() {
		if err != nil {
			undoErr := s.UndoReservation(ctx, rbdVol.JournalPool, rbdVol.Pool, rbdVol.RbdImageName, rbdVol.RequestName)
			if undoErr != nil {
				log.ErrorLog(ctx, "failed to undo reservation %s: %v", rbdVol, undoErr)
			}
		}
	}()
	rbdVol.VolID, err = util.GenerateVolID(ctx, rbdVol.Monitors, cr, imagePoolID, rbdVol.Pool, rbdVol.ClusterID, rbdVol.ReservedID, volIDVersion)
	if err != nil {
		return "", nil
	}

	log.DebugLog(ctx, "re-generated Volume ID (%s) and image name (%s) for request name (%s)",
		rbdVol.VolID, rbdVol.RbdImageName, rbdVol.RequestName)
	if rbdVol.ImageID == "" {
		err = rbdVol.storeImageID(ctx, s)
		if err != nil {
			return "", err
		}
	}

	return rbdVol.VolID, nil
}
