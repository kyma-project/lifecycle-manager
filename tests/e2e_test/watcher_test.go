//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"k8s.io/apimachinery/pkg/api/meta"
	"strings"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/controllers"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout      = 10 * time.Second
	readyTimeout = 30 * time.Second
	interval     = 1 * time.Second

	watcherPodContainer = "server"
	exampleSKRDomain    = "example.domain.com"

	KLMPodPrefix    = "klm-controller-manager"
	KLMPodContainer = "manager"

	controlPlaneNamespace = "kcp-system"
	dInDHostname          = "host.docker.internal"
	localHostname         = "0.0.0.0"
	k3dHostname           = "host.k3d.internal"
)

var (
	errPodNotFound               = errors.New("could not find pod")
	errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")
	errModuleNotExisting         = errors.New("module does not exists in KymaCR")
	errLogNotFound               = errors.New("logMsg was not found in log")
	errKymaNotReady              = errors.New("kyma CR not ready")
)

func resolveHostToBeReplaced() string {
	if isDinD {
		return dInDHostname
	} else {
		return localHostname
	}
}

var _ = Describe("Kyma CR change on runtime cluster triggers new reconciliation using the Watcher",
	Ordered, func() {
		channel := "regular"
		kymaName := "kyma-sample"
		kymaNamespace := "kcp-system"
		remoteNamespace := "kyma-system"
		incomingRequestMsg := fmt.Sprintf("event received from SKR, adding %s/%s to queue", kymaNamespace, kymaName)

		BeforeAll(func() {
			//make sure we can list Kymas to ensure CRDs have been installed
			err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
			Expect(meta.IsNoMatchError(err)).To(BeFalse())
		})

		It("Should create empty Kyma CR on remote cluster", func() {
			Eventually(createKymaCR, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, kymaNamespace, channel, controlPlaneClient).
				Should(Succeed())

			Eventually(createKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, kymaNamespace, channel, controlPlaneClient).
				Should(Succeed())
			By("verifying kyma is ready")
			Eventually(checkKymaReady, readyTimeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, kymaNamespace, controlPlaneClient).
				Should(Succeed())
			By("verifying remote kyma is ready")
			Eventually(checkRemoteKymaCR, timeout, interval).
				WithContext(ctx).
				WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient).
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
			Eventually(changeRemoteKymaChannel, timeout, interval).
				WithContext(ctx).
				WithArguments(remoteNamespace, "fast", runtimeClient).
				Should(Succeed())

			Eventually(checkKLMLogs, timeout, interval).
				WithContext(ctx).
				WithArguments(incomingRequestMsg, controlPlaneRESTConfig, runtimeRESTConfig, controlPlaneClient, runtimeClient).
				Should(Succeed())
		})

		It("Should delete Kyma CR on remote cluster", func() {
			Eventually(deleteKymaCR, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, kymaNamespace, channel, controlPlaneClient).
				Should(Succeed())

			Eventually(deleteKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kymaName, kymaNamespace, channel, controlPlaneClient).
				Should(Succeed())

			Eventually(checkRemoteKymaCRDeleted, timeout, interval).
				WithContext(ctx).
				WithArguments(remoteNamespace, runtimeClient).
				Should(Succeed())
		})
	})

func checkKymaReady(ctx context.Context, kymaName, kymaNamespace string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}
	if kyma.Status.State != v1beta2.StateReady {
		return errKymaNotReady
	}
	return nil
}

func createKymaCR(ctx context.Context, kymaName, kymaNamespace, channel string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: kymaNamespace,
			Labels: map[string]string{
				"operator.kyma-project.io/watched-by": "lifecycle-manager",
				"operator.kyma-project.io/sync":       "true",
			},
			Annotations: map[string]string{
				"operator.kyma-project.io/sync": "true",
				"skr-domain":                    exampleSKRDomain,
			},
		},
		Spec: v1beta2.KymaSpec{
			Channel: channel,
			Modules: nil,
		},
	}
	return k8sClient.Create(ctx, kyma)
}

func createKymaSecret(ctx context.Context, kymaName, kymaNamespace, channel string, k8sClient client.Client) error {
	hostnameToBeReplaced := resolveHostToBeReplaced()
	patchedRuntimeConfig := strings.ReplaceAll(string(*runtimeConfig), hostnameToBeReplaced, k3dHostname)
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
	return k8sClient.Create(ctx, secret)
}

func deleteKymaSecret(ctx context.Context, kymaName, kymaNamespace, channel string, k8sClient client.Client) error {
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, secret)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	Expect(err).ToNot(HaveOccurred())
	return k8sClient.Delete(ctx, secret)
}

func deleteKymaCR(ctx context.Context, kymaName, kymaNamespace, channel string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	Expect(err).ToNot(HaveOccurred())
	return k8sClient.Delete(ctx, kyma)
}

func checkRemoteKymaCR(ctx context.Context,
	kymaNamespace string, wantedModules []v1beta2.Module, k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: v1beta2.DefaultRemoteKymaName, Namespace: kymaNamespace}, kyma)
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

func checkRemoteKymaCRDeleted(ctx context.Context,
	kymaNamespace string, k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: v1beta2.DefaultRemoteKymaName, Namespace: kymaNamespace}, kyma)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func changeRemoteKymaChannel(ctx context.Context, kymaNamespace, channel string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: v1beta2.DefaultRemoteKymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}

	kyma.Spec.Channel = channel

	return k8sClient.Update(ctx, kyma)
}

func checkKLMLogs(ctx context.Context, logMsg string, controlPlaneConfig, runtimeConfig *rest.Config, k8sClient, runtimeClient client.Client) error {
	logs, err := getPodLogs(ctx, controlPlaneConfig, k8sClient, controlPlaneNamespace, KLMPodPrefix, KLMPodContainer)
	if err != nil {
		return err
	}

	GinkgoWriter.Printf("KLM Logs:  %s\n", logs)
	if strings.Contains(logs, logMsg) {
		return nil
	}

	watcherLogs, err := getPodLogs(ctx, runtimeConfig, runtimeClient, controllers.DefaultRemoteSyncNamespace, watcher.SkrResourceName, watcherPodContainer)
	if err != nil {
		return err
	}
	return fmt.Errorf("%w\n Expected: %s\n Given KLM logs: %s Watcher-Server-Logs: %s", errLogNotFound, logMsg, logs, watcherLogs)
}

func getPodLogs(ctx context.Context, config *rest.Config, k8sClient client.Client, namespace, podPrefix, container string) (string, error) {
	pod := &corev1.Pod{}
	podList := &corev1.PodList{}
	if err := k8sClient.List(ctx, podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return "", err
	}

	for _, p := range podList.Items {
		pod = &p
		GinkgoWriter.Printf("Found Pod:  %s/%s\n", pod.Namespace, pod.Name)
		if strings.HasPrefix(pod.Name, podPrefix) {
			GinkgoWriter.Printf("Pod has Prefix %s:  %s/%s\n", podPrefix, pod.Namespace, pod.Name)
			break
		}
	}
	if pod.Name == "" {
		return "", fmt.Errorf("%w: Prefix: %s Container: %s", errPodNotFound, podPrefix, container)
	}

	GinkgoWriter.Printf("Pod has prefix:  %s/%s\n", pod.Namespace, pod.Name)
	// temporary clientset, since controller-runtime client library does not support non-CRUD subresources
	// Open issue: https://github.com/kubernetes-sigs/controller-runtime/issues/452
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: container})
	GinkgoWriter.Printf("Request: %#v", req)
	GinkgoWriter.Printf("Request URL: %s", req.URL())
	podLogs, err := req.Stream(ctx)
	if err != nil {
		// In prow it errors here with
		// Error while stream %!w(*errors.StatusError=&{{{ } {   <nil>} Failure pods "skr-webhook-67cc9d96d7-2tczg" not found NotFound 0xc000513c20 404}})
		GinkgoWriter.Printf("Error while stream %w\n", err)
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		GinkgoWriter.Printf("Error while copy %w\n", err)
		return "", err
	}
	str := buf.String()

	return str, nil
}

func deleteWatcherDeployment(ctx context.Context, watcherName, watcherNamespace string, k8sClient client.Client) error {
	watcherDeployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watcherName,
			Namespace: watcherNamespace,
		},
	}
	return k8sClient.Delete(ctx, watcherDeployment)
}

func checkWatcherDeploymentReady(ctx context.Context,
	deploymentName, deploymentNamespace string, k8sClient client.Client,
) error {
	watcherDeployment := &v1.Deployment{}
	if err := k8sClient.Get(ctx,
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
	secretName, secretNamespace string, k8sClient client.Client,
) error {
	certificateSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	return k8sClient.Delete(ctx, certificateSecret)
}

func checkCertificateSecretExists(ctx context.Context,
	secretName, secretNamespace string, k8sClient client.Client,
) error {
	certificateSecret := &corev1.Secret{}
	return k8sClient.Get(ctx,
		client.ObjectKey{Name: secretName, Namespace: secretNamespace},
		certificateSecret,
	)
}
