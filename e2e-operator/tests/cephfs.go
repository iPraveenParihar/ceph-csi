package tests

import (
	"context"
	"github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CephFS Driver", Ordered, Label("cephfs"), func() {
	var ns string
	BeforeAll(func() {
		if !framework.Opts.TestCephFS {
			Skip("CephFS disabled")
		}
		var e error
		ns, e = framework.CreateTestNamespace(context.Background(), framework.Instance, "cephfs-test")
		Expect(e).NotTo(HaveOccurred())
	})
	AfterAll(func() {
		if ns != "" {
			Expect(framework.DeleteTestNamespace(context.Background(), framework.Instance, ns)).To(Succeed())
		}
	})
	It("shares a writable PVC between two pods", func() {
		
	})
})
