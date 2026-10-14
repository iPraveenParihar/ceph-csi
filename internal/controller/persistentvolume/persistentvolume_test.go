/*
Copyright 2025 The Ceph-CSI Authors.

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
package persistentvolume

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ctrl "github.com/ceph/ceph-csi/internal/controller"
)

func Test_shouldReconcileBasedOnDriver(t *testing.T) {
	t.Parallel()
	rbdDriver := "rbd.csi.ceph.com"
	type args struct {
		obj        client.Object
		driverName string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Nil object returns false",
			args: args{
				obj:        nil,
				driverName: rbdDriver,
			},
			want: false,
		},
		{
			name: "Object under deletion returns false",
			args: args{
				obj: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
				},
				driverName: rbdDriver,
			},
			want: false,
		},
		{
			name: "Missing annotation returns true",
			args: args{
				obj: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
				driverName: rbdDriver,
			},
			want: true,
		},
		{
			name: "Annotation matches driver returns true",
			args: args{
				obj: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"pv.kubernetes.io/provisioned-by": rbdDriver,
						},
					},
				},
				driverName: rbdDriver,
			},
			want: true,
		},
		{
			name: "Annotation does not match driver returns false",
			args: args{
				obj: &corev1.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"pv.kubernetes.io/provisioned-by": "cephfs.csi.ceph.com",
						},
					},
				},
				driverName: rbdDriver,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldReconcileBasedOnDriver(tt.args.obj, tt.args.driverName); got != tt.want {
				t.Errorf("shouldReconcileBasedOnDriver() = %v, want %v", got, tt.want)
			}
		})
	}
}

func vacName(name string) *string {
	return &name
}

func Test_getVolumeAttributesClassParameters(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	existingVAC := &storagev1.VolumeAttributesClass{
		ObjectMeta: metav1.ObjectMeta{Name: "qos-gold"},
		DriverName: "rbd.csi.ceph.com",
		Parameters: map[string]string{
			"maxReadIops":  "1000",
			"maxWriteIops": "2000",
		},
	}

	tests := []struct {
		name    string
		pv      *corev1.PersistentVolume
		want    map[string]string
		wantErr bool
	}{
		{
			name: "PV without VolumeAttributesClassName returns nil",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{VolumeAttributesClassName: nil},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "PV with empty VolumeAttributesClassName returns nil",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{VolumeAttributesClassName: vacName("")},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "PV referencing existing VolumeAttributesClass returns its parameters",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{
					VolumeAttributesClassName: vacName("qos-gold"),
					PersistentVolumeSource: corev1.PersistentVolumeSource{
						CSI: &corev1.CSIPersistentVolumeSource{Driver: "rbd.csi.ceph.com"},
					},
				},
			},
			want: map[string]string{
				"maxReadIops":  "1000",
				"maxWriteIops": "2000",
			},
			wantErr: false,
		},
		{
			name: "PV referencing missing VolumeAttributesClass returns error",
			pv: &corev1.PersistentVolume{
				Spec: corev1.PersistentVolumeSpec{VolumeAttributesClassName: vacName("does-not-exist")},
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(existingVAC.DeepCopy()).
				Build()
			r := &ReconcilePersistentVolume{client: cl}

			got, err := r.getVolumeAttributesClassParameters(context.TODO(), tt.pv)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getVolumeAttributesClassParameters() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("getVolumeAttributesClassParameters() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("getVolumeAttributesClassParameters()[%s] = %s, want %s", k, got[k], v)
				}
			}
		})
	}
}

type fakeDiffIterateReconciler struct {
	called bool
	err    error
}

func (f *fakeDiffIterateReconciler) ReconcileDiffIterateFeatures(_ context.Context) error {
	f.called = true

	return f.err
}

func Test_reconcileDiffIterateFeatures(t *testing.T) {
	t.Parallel()

	const driver = "rbd.csi.ceph.com"
	diffFeatures := "layering,deep-flatten,exclusive-lock,object-map,fast-diff"
	errReconcile := errors.New("reconcile failed")

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add scheme: %v", err)
	}

	newSC := func(name, provisioner, features string) *storagev1.StorageClass {
		return &storagev1.StorageClass{
			ObjectMeta:  metav1.ObjectMeta{Name: name},
			Provisioner: provisioner,
			Parameters:  map[string]string{imageFeaturesParam: features},
		}
	}
	newPV := func(scName, features string) *corev1.PersistentVolume {
		return &corev1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
			Spec: corev1.PersistentVolumeSpec{
				StorageClassName: scName,
				PersistentVolumeSource: corev1.PersistentVolumeSource{
					CSI: &corev1.CSIPersistentVolumeSource{
						Driver:           driver,
						VolumeAttributes: map[string]string{imageFeaturesParam: features},
					},
				},
			},
		}
	}

	tests := []struct {
		name          string
		pv            *corev1.PersistentVolume
		volErr        error
		wantReconcile bool
		wantEvent     bool
		wantErr       error
	}{
		{
			name:          "StorageClass with features reconciles the image chain",
			pv:            newPV("sc-diff", "layering"),
			wantReconcile: true,
		},
		{
			name:      "StorageClass without features alerts the admin",
			pv:        newPV("sc-layering", diffFeatures),
			wantEvent: true,
		},
		{
			name:          "deleted StorageClass falls back to the volume attributes",
			pv:            newPV("sc-deleted", diffFeatures),
			wantReconcile: true,
		},
		{
			name: "deleted StorageClass without features in volume attributes is skipped",
			pv:   newPV("sc-deleted", "layering"),
		},
		{
			name:          "StorageClass of another provisioner falls back to the volume attributes",
			pv:            newPV("sc-other", diffFeatures),
			wantReconcile: true,
		},
		{
			name:          "reconcile errors are returned for a retry",
			pv:            newPV("sc-diff", ""),
			volErr:        errReconcile,
			wantReconcile: true,
			wantErr:       errReconcile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cl := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(
					newSC("sc-diff", driver, diffFeatures),
					newSC("sc-layering", driver, "layering"),
					newSC("sc-other", "other.csi.example.com", "layering"),
				).
				Build()
			recorder := events.NewFakeRecorder(10)
			r := &ReconcilePersistentVolume{
				client:   cl,
				config:   ctrl.Config{DriverName: driver},
				recorder: recorder,
			}
			vol := &fakeDiffIterateReconciler{err: tt.volErr}

			err := r.reconcileDiffIterateFeatures(context.TODO(), tt.pv, vol)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("reconcileDiffIterateFeatures() error = %v, want %v", err, tt.wantErr)
			}
			if vol.called != tt.wantReconcile {
				t.Errorf("ReconcileDiffIterateFeatures() called = %t, want %t", vol.called, tt.wantReconcile)
			}
			if gotEvent := len(recorder.Events) > 0; gotEvent != tt.wantEvent {
				t.Errorf("event recorded = %t, want %t", gotEvent, tt.wantEvent)
			}
		})
	}
}
