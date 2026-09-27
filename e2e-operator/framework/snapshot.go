package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/wait"
)

func CreateVolumeSnapshot(ctx context.Context, fw *Framework, s *v1.VolumeSnapshot) (*v1.VolumeSnapshot, error) {
	return fw.SnapClient.SnapshotV1().VolumeSnapshots(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
}

func WaitForSnapshotReady(ctx context.Context, fw *Framework, ns, name string) error {
	return wait.PollUntilContextTimeout(ctx, time.Duration(2)*time.Second, Instance.DefaultTimeout, true, func(ctx context.Context) (bool, error) {
		s, e := fw.SnapClient.SnapshotV1().VolumeSnapshots(ns).Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(e) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return s.Status != nil && s.Status.ReadyToUse != nil && *s.Status.ReadyToUse, nil
	})
}

func DeleteVolumeSnapshot(ctx context.Context, fw *Framework, ns, name string) error {
	e := fw.SnapClient.SnapshotV1().VolumeSnapshots(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrs.IsNotFound(e) {
		return nil
	}
	if e != nil {
		return fmt.Errorf("delete snapshot: %w", e)
	}
	return nil
}

var _ = time.Second
