package tests

import (
	"flag"
	"os"
	"testing"

	"github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	framework.RegisterFlags()
	flag.Parse()
	os.Exit(m.Run())
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ceph-csi operator E2E")
}

var (
	_ = BeforeSuite(func() {
		var err error
		framework.Instance, err = framework.New()
		Expect(err).NotTo(HaveOccurred())
	})

	_ = ReportAfterEach(func(report SpecReport) {
		if report.Failed() {
			_ = framework.DumpPodLogs(GinkgoT().Context(), framework.Instance, framework.Opts.CephCSINamespace, "")
		}
	})
)
