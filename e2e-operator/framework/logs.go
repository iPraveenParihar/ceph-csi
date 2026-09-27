package framework

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DumpPodLogs(ctx context.Context, fw *Framework, namespace, labelSelector string) error {
	pods, e := fw.KubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if e != nil {
		return e
	}
	for _, p := range pods.Items {
		for _, c := range p.Spec.Containers {
			r, e := fw.KubeClient.CoreV1().Pods(namespace).GetLogs(p.Name, &corev1.PodLogOptions{Container: c.Name}).Do(ctx).Raw()
			if e != nil {
				fmt.Printf("logs %s/%s[%s]: %v\n", namespace, p.Name, c.Name, e)
				continue
			}
			fmt.Printf("logs %s/%s[%s]:\n%s\n", namespace, p.Name, c.Name, string(r))
		}
	}
	return nil
}
