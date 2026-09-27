package framework

import (
	"bytes"
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

func PodMountingPVC(namePrefix, namespace, pvcName string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{GenerateName: namePrefix + "-", Namespace: namespace}, Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "work", Image: "busybox:1.36", Command: []string{"sh", "-c", "sleep 36000"}, VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: "/data"}}}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}}}
}

func PodWithBlockDevice(namePrefix, namespace, pvcName, devicePath string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{GenerateName: namePrefix + "-", Namespace: namespace}, Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "work", Image: "busybox:1.36", Command: []string{"sh", "-c", "sleep 36000"}, VolumeDevices: []corev1.VolumeDevice{{Name: "data", DevicePath: devicePath}}}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvcName}}}}}}
}

func CreatePod(ctx context.Context, fw *Framework, p *corev1.Pod) (*corev1.Pod, error) {
	return fw.KubeClient.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{})
}

func DeletePod(ctx context.Context, fw *Framework, ns, name string) error {
	e := fw.KubeClient.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrs.IsNotFound(e) {
		return nil
	}
	return e
}

func WaitForPodRunning(ctx context.Context, fw *Framework, ns, name string) error {
	return wait.PollUntilContextTimeout(ctx, time.Duration(2)*time.Second, Instance.DefaultTimeout, true, func(ctx context.Context) (bool, error) {
		p, e := fw.KubeClient.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(e) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return p.Status.Phase == corev1.PodRunning, nil
	})
}

func ExecInPod(ctx context.Context, namespace, podName string, command []string) (string, error) {
	pod, err := Instance.KubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get pod %s/%s for exec: %w", namespace, podName, err)
	}
	if len(pod.Spec.Containers) == 0 {
		return "", fmt.Errorf("pod %s/%s has no containers", namespace, podName)
	}
	container := pod.Spec.Containers[0].Name
	for _, candidate := range pod.Spec.Containers {
		if candidate.Name == "work" {
			container = candidate.Name
			break
		}
	}
	req := Instance.KubeClient.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace(namespace).SubResource("exec").VersionedParams(&corev1.PodExecOptions{Container: container, Command: command, Stdout: true, Stderr: true}, scheme.ParameterCodec)
	ex, e := remotecommand.NewSPDYExecutor(Instance.Config, "POST", req.URL())
	if e != nil {
		return "", e
	}
	var out, errOut bytes.Buffer
	e = ex.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &out, Stderr: &errOut})
	if e != nil {
		return out.String(), fmt.Errorf("exec failed: %w: %s", e, errOut.String())
	}
	return out.String(), nil
}

func execWithRetry(ctx context.Context, opts *e2epod.ExecOptions) (string, string, error) {
	var (
		stdOut, stdErr string
		err            error
	)
	// err := wait.PollUntilContextTimeout(context.TODO(), poll, timeout, true, func(_ context.Context) (bool, error) {
	// 	var execErr error
	// 	stdOut, stdErr, execErr = e2epod.ExecWithOptions(f, *opts)
	// 	if execErr != nil {
	// 		if isRetryableAPIError(execErr) {
	// 			return false, nil
	// 		}

	// 		framework.Logf("failed to execute command: %v", execErr)

	// 		return false, fmt.Errorf("failed to execute command: %w", execErr)
	// 	}

	// 	return true, nil
	// })

	err = wait.PollUntilContextTimeout(ctx, time.Duration(2)*time.Second, Instance.DefaultTimeout, true, func(_ context.Context) (bool, error) {
		stdOut, err = ExecInPod(ctx, opts.Namespace, opts.PodName, opts.Command)
		if err != nil {
			if IsRetryableAPIError(err) {
				return false, nil
			}

			return false, fmt.Errorf("failed to execute command: %w", err)
		}

		return true, nil
	})

	return stdOut, stdErr, err
}
