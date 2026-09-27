package tests

import (
	"context"
	"fmt"
	"strings"

	f "github.com/ceph/ceph-csi/ceph-csi-e2e/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rookToolBoxPodLabel = "app=rook-ceph-tools"
)

var clusterID string

func getClusterID(ctx context.Context) (string, error) {
	if clusterID != "" {
		return clusterID, nil
	}

	fsID, _, err := execInToolsPod(ctx, []string{"ceph", "fsid"})
	if err != nil {
		return clusterID, err
	}
	clusterID = strings.Trim(fsID, "\n")

	return clusterID, nil
}

func execInToolsPod(ctx context.Context, command []string) (string, string, error) {
	// pods, e := f.Instance.KubeClient.CoreV1().Pods(f.Opts.RookNamespace).List(
	// 	ctx, metav1.ListOptions{LabelSelector: "app=rook-ceph-tools"},
	// )
	// if e != nil {
	// 	return "", e
	// }
	// if len(pods.Items) == 0 {
	// 	return "", fmt.Errorf("rook-ceph-tools pod not found in %s", f.Opts.RookNamespace)
	// }
	// return f.ExecInPod(ctx, f.Opts.RookNamespace, pods.Items[0].Name, command)

	opt := &metav1.ListOptions{
		LabelSelector: rookToolBoxPodLabel,
	}
	podOpt, err := getCommandInPodOpts(f, c, ns, "", opt)
	if err != nil {
		return "", "", err
	}

	stdOut, stdErr, err := execWithRetry(f, &podOpt)
	if stdErr != "" {
		framework.Logf("stdErr occurred: %v", stdErr)
	}

	return stdOut, stdErr, err


}
