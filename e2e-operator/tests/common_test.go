package tests

import (
	"context"

	"github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Cluster health", func() {
	It("can reach the Kubernetes API", func() {
		_, err := framework.Instance.KubeClient.Discovery().ServerVersion()
		Expect(err).NotTo(HaveOccurred())
	})
	It("can list cluster namespaces", func() {
		_, err := framework.Instance.KubeClient.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
		Expect(err).NotTo(HaveOccurred())
	})
})
