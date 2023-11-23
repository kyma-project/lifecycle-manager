package testutils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

const (
	controlPlaneNamespace = "kcp-system"
	watcherPodContainer   = "server"
	KLMPodPrefix          = "klm-controller-manager"
	KLMPodContainer       = "manager"
	remoteNamespace       = "kyma-system"
)

var (
	ErrPodNotFound = errors.New("could not find pod")
	ErrLogNotFound = errors.New("logMsg was not found in log")
)

func CheckKLMLogs(ctx context.Context,
	logMsg string,
	controlPlaneConfig, runtimeConfig *rest.Config,
	k8sClient, runtimeClient client.Client,
	logsSince *apimetav1.Time,
) error {
	logs, err := getPodLogs(ctx, controlPlaneConfig, k8sClient, controlPlaneNamespace, KLMPodPrefix, KLMPodContainer,
		logsSince)
	if err != nil {
		return err
	}

	if strings.Contains(logs, logMsg) {
		return nil
	}

	_, err = getPodLogs(ctx, runtimeConfig,
		runtimeClient, remoteNamespace, watcher.SkrResourceName, watcherPodContainer, logsSince)

	if err != nil {
		return err
	}
	return ErrLogNotFound
}

func getPodLogs(ctx context.Context,
	config *rest.Config,
	k8sClient client.Client,
	namespace, podPrefix, container string,
	logsSince *apimetav1.Time,
) (string, error) {
	pod := apicorev1.Pod{}
	podList := &apicorev1.PodList{}
	if err := k8sClient.List(ctx, podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return "", fmt.Errorf("failed to list pods %w", err)
	}

	for i := range podList.Items {
		pod = podList.Items[i]
		if strings.HasPrefix(pod.Name, podPrefix) {
			break
		}
	}
	if pod.Name == "" {
		return "", fmt.Errorf("%w: Prefix: %s Container: %s", ErrPodNotFound, podPrefix, container)
	}

	// temporary clientset, since controller-runtime client library does not support non-CRUD subresources
	// Open issue: https://github.com/kubernetes-sigs/controller-runtime/issues/452
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create clientset, %w", err)
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apicorev1.PodLogOptions{
		Container: container,
		SinceTime: logsSince,
	})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get pod logs %w", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to copy pod logs %w", err)
	}
	str := buf.String()

	return str, nil
}
