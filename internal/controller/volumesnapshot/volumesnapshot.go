/*
Copyright 2024 The Ceph-CSI Authors.

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
package volumesnapshot

import (
	"context"
	"errors"
	"fmt"

	ctrl "github.com/ceph/ceph-csi/internal/controller"
	"github.com/ceph/ceph-csi/internal/rbd"
	"github.com/ceph/ceph-csi/internal/util/log"

	"github.com/ceph/ceph-csi/internal/util"

	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ReconcileVolumeSnapshot struct {
	client client.Client
	config ctrl.Config
	Locks  *util.VolumeLocks
}

var (
	_ reconcile.Reconciler = &ReconcileVolumeSnapshot{}
	_ ctrl.Manager         = &ReconcileVolumeSnapshot{}
)

// Init will add the ReconcilePersistentVolume to the list.
func Init() {
	ctrl.ControllerList = append(ctrl.ControllerList, &ReconcileVolumeSnapshot{})
}

// Add adds the newVSReconciler.
func (r *ReconcileVolumeSnapshot) Add(mgr manager.Manager, config ctrl.Config) error {
	return add(mgr, newVSReconciler(mgr, config))
}

// newVSReconciler returns a ReconcilePersistentVolume.
func newVSReconciler(mgr manager.Manager, config ctrl.Config) reconcile.Reconciler {
	r := &ReconcileVolumeSnapshot{
		client: mgr.GetClient(),
		config: config,
		Locks:  util.NewVolumeLocks(),
	}

	return r
}

func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(
		"volumesnapshot-controller",
		mgr,
		controller.Options{MaxConcurrentReconciles: 1, Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to VolumeSnapshotContent
	err = c.Watch(source.Kind(mgr.GetCache(), &snapapi.VolumeSnapshotContent{}), &handler.EnqueueRequestForObject{})
	if err != nil {
		return fmt.Errorf("failed to watch the changes: %w", err)
	}

	return nil
}

func (r *ReconcileVolumeSnapshot) getCredentials(
	ctx context.Context,
	name,
	namespace string,
) (*util.Credentials, error) {
	var cr *util.Credentials

	if name == "" || namespace == "" {
		errStr := "secret name or secret namespace is empty"
		log.ErrorLogMsg(errStr)

		return nil, errors.New(errStr)
	}
	secret := &corev1.Secret{}
	err := r.client.Get(ctx,
		types.NamespacedName{Name: name, Namespace: namespace},
		secret)
	if err != nil {
		return nil, fmt.Errorf("error getting secret %s in namespace %s: %w", name, namespace, err)
	}

	credentials := map[string]string{}
	for key, value := range secret.Data {
		credentials[key] = string(value)
	}

	cr, err = util.NewUserCredentials(credentials)
	if err != nil {
		log.ErrorLogMsg("failed to get user credentials %s", err)

		return nil, err
	}

	return cr, nil
}

func (r *ReconcileVolumeSnapshot) reconcileVS(ctx context.Context, obj runtime.Object) error {
	vsc, ok := obj.(*snapapi.VolumeSnapshotContent)
	if !ok {
		return nil
	}
	if vsc.Spec.Driver != r.config.DriverName {
		return nil
	}
	if *vsc.Spec.Source.SnapshotHandle == "" {
		return nil
	}

	AnnDeletionSecretRefName := "snapshot.storage.kubernetes.io/deletion-secret-name"
	AnnDeletionSecretRefNamespace := "snapshot.storage.kubernetes.io/deletion-secret-namespace"

	vscNamespace := vsc.Namespace
	requestName := vsc.Name
	snapshotHandleName := vsc.Spec.Source.SnapshotHandle
	// sourceVolumeHandleName := vsc.Spec.Source.SnapshotHandle

	secretName, _ := vsc.Annotations[AnnDeletionSecretRefName]
	secretNamespace, _ := vsc.Annotations[AnnDeletionSecretRefNamespace]

	if ok := r.Locks.TryAcquire(*vsc.Status.SnapshotHandle); !ok {
		return fmt.Errorf("")
	}
	defer r.Locks.Release(*vsc.Status.SnapshotHandle)

	creds, err := r.getCredentials(ctx, secretName, secretNamespace)
	if err != nil {
		log.ErrorLogMsg("failed to get credentials from secret %s", err)
		return err
	}
	defer creds.DeleteCredentials()

	rbdVolID, err := rbd.RegenerateSnapJournal(
		*snapshotHandleName,
		r.config.ClusterName,
		vscNamespace,
		requestName,
		creds,
	)
	if err != nil {
		log.ErrorLogMsg("failed to regenerate journal %s", err)

		return err
	}
	if rbdVolID != *snapshotHandleName {
		log.DebugLog(ctx, "volumeHandler changed from %s to %s", *snapshotHandleName, rbdVolID)
	}

	return nil
}

func (r *ReconcileVolumeSnapshot) Reconcile(ctx context.Context,
	request reconcile.Request,
) (reconcile.Result, error) {
	vsc := &snapapi.VolumeSnapshotContent{}
	err := r.client.Get(ctx, request.NamespacedName, vsc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}
	// Check if the object is under deletion
	if !vsc.GetDeletionTimestamp().IsZero() {
		return reconcile.Result{}, nil
	}

	err = r.reconcileVS(ctx, vsc)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}
