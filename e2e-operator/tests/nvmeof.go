package tests

import (
	"context"
	"github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NVMe-oF Driver", Ordered, Label("nvmeof"), func() {
	var ns string
	BeforeAll(func() {
		if !framework.Opts.TestNVMeoF {
			Skip("NVMe-oF disabled")
		}
		var e error
		ns, e = framework.CreateTestNamespace(context.Background(), framework.Instance, "nvmeof-test")
		Expect(e).NotTo(HaveOccurred())
	})
	AfterAll(func() {
		if ns != "" {
			Expect(framework.DeleteTestNamespace(context.Background(), framework.Instance, ns)).To(Succeed())
		}
	})
	It("writes to a raw block PVC", func() {
		
	})
})
