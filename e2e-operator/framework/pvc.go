package framework

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func CreatePVC(ctx context.Context, fw *Framework, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {
	return fw.KubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
}

func WaitForPVCBound(ctx context.Context, fw *Framework, namespace, name string) error {
	return wait.PollUntilContextTimeout(ctx, time.Duration(2) * time.Second, Instance.DefaultTimeout, true, func(ctx context.Context) (bool, error) {
		p, e := fw.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(e) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return p.Status.Phase == corev1.ClaimBound, nil
	})
}

func DeletePVC(ctx context.Context, fw *Framework, namespace, name string) error {
	e := fw.KubeClient.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrs.IsNotFound(e) {
		return nil
	}
	return e
}

func PVC(name, namespace, storageClass string, mode corev1.PersistentVolumeMode, access corev1.PersistentVolumeAccessMode) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}, Spec: corev1.PersistentVolumeClaimSpec{StorageClassName: &storageClass, VolumeMode: &mode, AccessModes: []corev1.PersistentVolumeAccessMode{access}, Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: mustQuantity("1Gi")}}}}
}

func mustQuantity(s string) resource.Quantity {
	q, e := resource.ParseQuantity(s)
	if e != nil {
		panic(fmt.Sprint(e))
	}
	return q
}
