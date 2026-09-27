package framework

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateTestNamespace(ctx context.Context, fw *Framework, prefix string) (string, error) {
	ns, err := fw.KubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: prefix + "-"}}, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return ns.Name, nil
}
func DeleteTestNamespace(ctx context.Context, fw *Framework, name string) error {
	return fw.KubeClient.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
}
