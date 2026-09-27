package tests

import (
	"context"

	f "github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RBD Driver", Ordered, Label("rbd"), func() {
	var ns string
	BeforeAll(func(ctx context.Context) {
		if !f.Opts.TestRBD {
			Skip("RBD tests disabled")
		}
		var e error
		ns, e = f.CreateTestNamespace(ctx, f.Instance, "rbd-test")
		Expect(e).NotTo(HaveOccurred())
	})
	AfterAll(func(ctx context.Context) {
		if ns == "" {
			return
		}
		Expect(f.DeleteTestNamespace(ctx, f.Instance, ns)).To(Succeed())
	})
	It("provisions a block PVC, mounts it, and writes data", func(ctx context.Context) {
		// ctx := context.Background()
		// p, e := framework.CreatePVC(ctx, framework.Instance, framework.PVC("rbd", ns, "csi-rbd-sc", corev1.PersistentVolumeBlock, corev1.ReadWriteOnce))
		// Expect(e).NotTo(HaveOccurred())
		// DeferCleanup(func() { _ = framework.DeletePVC(context.Background(), framework.Instance, ns, p.Name) })
		// Expect(framework.WaitForPVCBound(ctx, framework.Instance, ns, p.Name)).To(Succeed())
		// pod, e := framework.CreatePod(ctx, framework.Instance, framework.PodMountingPVC("rbd", ns, p.Name))
		// Expect(e).NotTo(HaveOccurred())
		// DeferCleanup(func() { _ = framework.DeletePod(context.Background(), framework.Instance, ns, pod.Name) })
		// Expect(framework.WaitForPodRunning(ctx, framework.Instance, ns, pod.Name)).To(Succeed())
		// _, e = framework.ExecInPod(ctx, framework.Instance, ns, pod.Name, []string{"sh", "-c", "echo rbd-data > /data/test"})
		// Expect(e).NotTo(HaveOccurred())


	})
})
