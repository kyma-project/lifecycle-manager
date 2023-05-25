//go:build e2e
// +build e2e

package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout      = 10 * time.Second
	readyTimeout = 30 * time.Second
	interval     = 1 * time.Second
)

var (
	errKLMPodNotFound            = errors.New("could not find KLM pod")
	errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")
	errModuleNotExisting         = errors.New("module does not exists in KymaCR")
	errLogNotFound               = errors.New("logMsg was not found in log")
)

var _ = Describe("Kyma CR change on runtime cluster triggers new reconciliation using the Watcher",
	Ordered, func() {
		channel := "regular"
		kymaName := "kyma-sample"
		kymaNamespace := "kcp-system"
		remoteNamespace := "kyma-system"
		incomingRequestMsg := fmt.Sprintf("event coming from SKR, adding %s/%s to queue", kymaNamespace, kymaName)

		It("Should create empty Kyma CR on remote cluster", func() {
			Eventually(createKymaCR, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, kymaNamespace, channel, controlPlaneClient).
				Should(Succeed())

			Eventually(checkRemoteKymaCR, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, remoteNamespace, []v1beta2.Module{}, runtimeClient).
				Should(Succeed())
		})

		It("Should redeploy watcher if it is deleted on remote cluster", func() {
			Eventually(checkWatcherDeploymentReady, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
				Should(Succeed())
			Eventually(deleteWatcherDeployment, timeout, interval).
				WithContext(ctx).
				WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
				Should(Succeed())
			Eventually(checkWatcherDeploymentReady, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
				Should(Succeed())
		})

		It("Should redeploy certificates if deleted on remote cluster", func() {
			Eventually(checkCertificateSecretExists, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(watcher.SkrTLSName, remoteNamespace, runtimeClient).
				Should(Succeed())
			Eventually(deleteCertificateSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(watcher.SkrTLSName, remoteNamespace, runtimeClient).
				Should(Succeed())
			Eventually(checkCertificateSecretExists, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(watcher.SkrTLSName, remoteNamespace, runtimeClient).
				Should(Succeed())
		})

		It("Should trigger new reconciliation", func() {
			Eventually(changeKymaCRChannel, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, remoteNamespace, "fast", runtimeClient).
				Should(Succeed())

			Eventually(checkKLMLogs, timeout, interval).
				WithContext(ctx).
				WithArguments(incomingRequestMsg, controlPlaneRESTConfig, controlPlaneClient).
				Should(Succeed())
			test := true
			Expect(test).Should(BeTrue())
		})
	})

func createKymaCR(ctx context.Context, kymaName, kymaNamespace, channel string, k8sclient client.Client) error {
	patchedRuntimeConfig := strings.ReplaceAll(string(*runtimeConfig), "0.0.0.0", "host.k3d.internal")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: kymaNamespace,
			Labels: map[string]string{
				"operator.kyma-project.io/kyma-name":  "kyma-sample",
				"operator.kyma-project.io/managed-by": "lifecycle-manager",
			},
		},
		Data: map[string][]byte{"config": []byte(patchedRuntimeConfig)},
	}
	err := k8sclient.Create(ctx, secret)
	if err != nil {
		return err
	}

	kyma := &v1beta2.Kyma{
		ObjectMeta: metav1.ObjectMeta{
			Name:        kymaName,
			Namespace:   kymaNamespace,
			Annotations: map[string]string{"skr-domain": "example.domain.com"},
		},
		Spec: v1beta2.KymaSpec{
			Channel: channel,
			Modules: nil,
		},
	}
	err = k8sclient.Create(ctx, kyma)
	if err != nil {
		return err
	}
	return nil
}

func checkRemoteKymaCR(ctx context.Context,
	kymaName, kymaNamespace string, wantedModules []v1beta2.Module, k8sclient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sclient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return err
	}

	for _, wantedModule := range wantedModules {
		exists := false
		for _, givenModule := range kyma.Spec.Modules {
			if givenModule.Name == wantedModule.Name &&
				givenModule.Channel == wantedModule.Channel {
				exists = true
				break
			}
		}
		if !exists {
			return fmt.Errorf("%w: %s/%s", errModuleNotExisting, wantedModule.Name, wantedModule.Channel)
		}
	}

	return nil
}

func changeKymaCRChannel(ctx context.Context,
	kymaName, kymaNamespace string, channel string, k8sclient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sclient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}

	kyma.Spec.Channel = channel

	return k8sclient.Update(ctx, kyma)
}

func checkKLMLogs(ctx context.Context, logMsg string, config *rest.Config, k8sClient client.Client) error {
	klmPod := &corev1.Pod{}
	podList := &corev1.PodList{}
	if err := k8sClient.List(ctx, podList, &client.ListOptions{Namespace: "kcp-system"}); err != nil {
		return err
	}

	for _, pod := range podList.Items {
		pod := pod
		if strings.HasPrefix(pod.Name, "klm-controller-manager") {
			klmPod = &pod
			break
		}
	}
	if klmPod.Name == "" {
		return errKLMPodNotFound
	}

	logs, err := getPodLogs(ctx, config, klmPod)
	if err != nil {
		return err
	}

	if strings.Contains(logs, logMsg) {
		return nil
	}

	return fmt.Errorf("%w: %s", errLogNotFound, logMsg)
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

func deleteWatcherDeployment(ctx context.Context, watcherName, watcherNamespace string, k8sclient client.Client) error {
	watcherDeployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watcherName,
			Namespace: watcherNamespace,
		},
	}
	return k8sclient.Delete(ctx, watcherDeployment)
}

func checkWatcherDeploymentReady(ctx context.Context,
	deploymentName, deploymentNamespace string, k8sclient client.Client,
) error {
	watcherDeployment := &v1.Deployment{}
	if err := k8sclient.Get(ctx,
		client.ObjectKey{Name: deploymentName, Namespace: deploymentNamespace},
		watcherDeployment,
	); err != nil {
		return err
	}

	if watcherDeployment.Status.ReadyReplicas != 1 {
		return fmt.Errorf("%w: %s/%s", errWatcherDeploymentNotReady, deploymentNamespace, deploymentName)
	}

	return nil
}

func deleteCertificateSecret(ctx context.Context,
	secretName, secretNamespace string, k8sclient client.Client,
) error {
	certificateSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	return k8sclient.Delete(ctx, certificateSecret)
}

func checkCertificateSecretExists(ctx context.Context,
	secretName, secretNamespace string, k8sclient client.Client,
) error {
	certificateSecret := &corev1.Secret{}
	return k8sclient.Get(ctx,
		client.ObjectKey{Name: secretName, Namespace: secretNamespace},
		certificateSecret,
	)
}
