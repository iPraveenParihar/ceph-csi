package framework

import "flag"

type TestOptions struct {
	TestRBD, TestNFS, TestCephFS, TestNVMeoF    bool
	CephCSINamespace, RookNamespace, Kubeconfig string
	DefaultTimeout                              int
}

var Opts TestOptions

func RegisterFlags() {
	flag.BoolVar(&Opts.TestRBD, "test-rbd", true, "run RBD tests")
	flag.BoolVar(&Opts.TestNFS, "test-nfs", true, "run NFS tests")
	flag.BoolVar(&Opts.TestCephFS, "test-cephfs", true, "run CephFS tests")
	flag.BoolVar(&Opts.TestNVMeoF, "test-nvmeof", true, "run NVMe-oF tests")
	flag.StringVar(&Opts.CephCSINamespace, "ceph-csi-namespace", "ceph-csi-operator-system", "ceph-csi namespace")
	flag.StringVar(&Opts.RookNamespace, "rook-namespace", "rook-ceph", "Rook namespace")
	flag.StringVar(&Opts.Kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flag.IntVar(&Opts.DefaultTimeout, "default-timeout", 300, "default timeout in seconds")
}
