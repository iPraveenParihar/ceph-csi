package tests

import (
	f "github.com/ceph/ceph-csi/ceph-csi-e2e/framework"

	storagev1 "k8s.io/api/storage/v1"
)

const (
	rbdDriverName    = "rbd.csi.com"
	rbdDefaultSCName = "csi-rbd-sc"

	rbdProvisionerSecretName = "cephcsi-rbd-provisioner"
	rbdNodePluginSecretName  = "cephcsi-rbd-node"
)

func RBDStorageClass(scopts f.StorageClassOpts) storagev1.StorageClass {
	sc := storagev1.StorageClass{}

	sc.Parameters = map[string]string{
		"csi.storage.k8s.io/provisioner-secret-namespace":        f.Instance.CephCSINamespace,
		"csi.storage.k8s.io/provisioner-secret-name":             rbdProvisionerSecretName,
		"csi.storage.k8s.io/controller-expand-secret-namespace":  f.Instance.CephCSINamespace,
		"csi.storage.k8s.io/controller-expand-secret-name":       rbdProvisionerSecretName,
		"csi.storage.k8s.io/controller-publish-secret-namespace": f.Instance.CephCSINamespace,
		"csi.storage.k8s.io/controller-publish-secret-name":      rbdProvisionerSecretName,
		"csi.storage.k8s.io/node-stage-secret-namespace":         f.Instance.CephCSINamespace,
		"csi.storage.k8s.io/node-stage-secret-name":              rbdNodePluginSecretName,
	}

	return sc
}