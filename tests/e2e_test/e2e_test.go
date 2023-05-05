package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"
)

const (
	timeout  = 10 * time.Second
	interval = 1 * time.Second
)

var _ = Describe("Kyma CR change on runtime cluster triggers new reconciliation using the Watcher", Ordered, func() {
	It("Should trigger new reconciliation", func() {

		logMsg := "dispatched event"
		Eventually(changeKymaCRChannel, timeout, interval).
			WithContext(ctx).
			WithArguments("kyma-sample", "kyma-system", "fast", runtimeClient).
			Should(Succeed())

		Eventually(checkKLMLogs, timeout, interval).
			WithContext(ctx).
			WithArguments(logMsg, controlPlaneRESTConfig, controlPlaneClient).
			Should(Succeed())
		test := true
		Expect(test).Should(BeTrue())
	})
})

func changeKymaCRChannel(ctx context.Context, kymaName, kymaNamespace string, channel string, k8sclient client.Client) error {
	kyma := &v1beta1.Kyma{}
	err := k8sclient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return err
	}

	kyma.Spec.Channel = channel
	err = k8sclient.Update(ctx, kyma)
	if err != nil {
		return err
	}

	return nil
}

func checkKLMLogs(ctx context.Context, logMsg string, config *rest.Config, k8sClient client.Client) error {
	klmPod := &corev1.Pod{}
	podList := &corev1.PodList{}
	if err := k8sClient.List(ctx, podList, &client.ListOptions{Namespace: "kcp-system"}); err != nil {
		return err
	}

	for _, pod := range podList.Items {
		if strings.HasPrefix(pod.Name, "lifecycle-manager-controller-manager") {
			klmPod = &pod
			break
		}
	}
	if klmPod.Name == "" {
		return errors.New("could not find KLM pod")
	}

	logs, err := getPodLogs(ctx, config, klmPod)
	if err != nil {
		return err
	}

	if strings.Contains(logs, logMsg) {
		return nil
	}

	return errors.New(fmt.Sprintf("LogMsg '%s' was not found in log.", logMsg))
}

func getPodLogs(ctx context.Context, config *rest.Config, pod *corev1.Pod) (string, error) {
	// temporary clientset, since controller-runtime client library does not support non-CRUD subresources
	// Open issue: https://github.com/kubernetes-sigs/controller-runtime/issues/452
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "manager"})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	str := buf.String()

	return str, nil
}
