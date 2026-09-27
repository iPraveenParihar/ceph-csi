package tests

import (
	"context"
	"github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NFS Driver", Ordered, Label("nfs"), func() {
	var ns string
	BeforeAll(func() {
		if !framework.Opts.TestNFS {
			Skip("NFS disabled")
		}
		var e error
		ns, e = framework.CreateTestNamespace(context.Background(), framework.Instance, "nfs-test")
		Expect(e).NotTo(HaveOccurred())
	})
	AfterAll(func() {
		if ns != "" {
			Expect(framework.DeleteTestNamespace(context.Background(), framework.Instance, ns)).To(Succeed())
		}
	})
	It("preserves data across pod recreation", func() {
		
	})
})
