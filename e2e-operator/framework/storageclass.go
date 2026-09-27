package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
)

type StorageClassOpts struct {
	Name              string
	Provisioner       string
	Parameters        map[string]string
	VolumeBindingMode *storagev1.VolumeBindingMode
	ReclaimPolicy     *corev1.PersistentVolumeReclaimPolicy
	MountOptions      []string
}

// CreateStorageClass creates a StorageClass and retries transient API errors
// until the configured default timeout expires.
func CreateStorageClass(ctx context.Context, fw *Framework, sc *storagev1.StorageClass) (*storagev1.StorageClass, error) {
	if fw == nil || fw.KubeClient == nil {
		return nil, fmt.Errorf("Kubernetes client is nil")
	}
	if sc == nil {
		return nil, fmt.Errorf("StorageClass is nil")
	}

	var created *storagev1.StorageClass
	err := wait.PollUntilContextTimeout(ctx, time.Duration(2) * time.Second, Instance.DefaultTimeout, true, func(ctx context.Context) (bool, error) {
		var err error
		created, err = fw.KubeClient.StorageV1().StorageClasses().Create(ctx, sc, metav1.CreateOptions{})
		if err == nil {
			return true, nil
		}
		if isRetryableStorageClassError(err) {
			return false, nil
		}

		return false, err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create StorageClass %q: %w", sc.Name, err)
	}

	return created, nil
}

func GetStorageClass(ctx context.Context, fw *Framework, name string) (*storagev1.StorageClass, error) {
	return fw.KubeClient.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
}

// DeleteStorageClass deletes a StorageClass, succeeding when it is already absent.
func DeleteStorageClass(ctx context.Context, fw *Framework, name string) error {
	if fw == nil || fw.KubeClient == nil {
		return fmt.Errorf("Kubernetes client is nil")
	}
	err := wait.PollUntilContextTimeout(ctx, time.Duration(2) * time.Second, Instance.DefaultTimeout, true, func(ctx context.Context) (bool, error) {
		err := fw.KubeClient.StorageV1().StorageClasses().Delete(ctx, name, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if isRetryableStorageClassError(err) {
			return false, nil
		}

		return err == nil, err
	})
	if err != nil {
		return fmt.Errorf("failed to delete StorageClass %q: %w", name, err)
	}

	return nil
}

func isRetryableStorageClassError(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsInternalError(err) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) ||
		apierrors.IsTooManyRequests(err) || utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) ||
		utilnet.IsConnectionRefused(err) {
		return true
	}
	if _, shouldRetry := apierrors.SuggestsClientDelay(err); shouldRetry {
		return true
	}
	return strings.Contains(err.Error(), "etcdserver: request timed out")
}
