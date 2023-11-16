package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	apiappsv1 "k8s.io/api/apps/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	watcherPodContainer = "server"
	KLMPodPrefix        = "klm-controller-manager"
	KLMPodContainer     = "manager"
	watcherCrName       = "kyma-watcher"
)

var (
	errPodNotFound               = errors.New("could not find pod")
	errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")
	errLogNotFound               = errors.New("logMsg was not found in log")
)

var _ = Describe("Enqueue Event from Watcher", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)
	incomingRequestMsg := fmt.Sprintf("event received from SKR, adding %s/%s to queue",
		kyma.GetNamespace(), kyma.GetName())

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	It("Should redeploy watcher if it is deleted on remote cluster", func() {
		By("verifying Runtime-Watcher is ready")
		Eventually(checkWatcherDeploymentReady).
			WithContext(ctx).
			WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
			Should(Succeed())
		By("deleting Runtime-Watcher deployment")
		Eventually(deleteWatcherDeployment).
			WithContext(ctx).
			WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
			Should(Succeed())
		By("verifying Runtime-Watcher deployment will be reapplied and becomes ready")
		Eventually(checkWatcherDeploymentReady).
			WithContext(ctx).
			WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
			Should(Succeed())
	})

	It("Should redeploy certificates if deleted on remote cluster", func() {
		By("verifying certificate secret exists on remote cluster")
		Eventually(checkCertificateSecretExists).
			WithContext(ctx).
			WithArguments(watcher.SkrTLSName, remoteNamespace, runtimeClient).
			Should(Succeed())
		By("Deleting certificate secret on remote cluster")
		Eventually(deleteCertificateSecret).
			WithContext(ctx).
			WithArguments(watcher.SkrTLSName, remoteNamespace, runtimeClient).
			Should(Succeed())
		By("verifying certificate secret will be recreated on remote cluster")
		Eventually(checkCertificateSecretExists).
			WithContext(ctx).
			WithArguments(watcher.SkrTLSName, remoteNamespace, runtimeClient).
			Should(Succeed())
	})

	It("Should trigger new reconciliation by spec watching", func() {
		By("changing the spec of the remote KymaCR")
		timeNow := &apimetav1.Time{Time: time.Now()}
		GinkgoWriter.Println(fmt.Sprintf("Spec watching logs since %s: ", timeNow))
		switchedChannel := "fast"
		Eventually(changeRemoteKymaChannel).
			WithContext(ctx).
			WithArguments(remoteNamespace, switchedChannel, runtimeClient).
			Should(Succeed())
		By("verifying new reconciliation got triggered for corresponding KymaCR on KCP")
		Eventually(checkKLMLogs).
			WithContext(ctx).
			WithArguments(incomingRequestMsg, controlPlaneRESTConfig, runtimeRESTConfig, controlPlaneClient,
				runtimeClient, timeNow).
			Should(Succeed())
	})

	It("Should trigger new reconciliation by status sub-resource watching", func() {
		time.Sleep(1 * time.Second)
		patchingTimestamp := &apimetav1.Time{Time: time.Now()}
		GinkgoWriter.Println(fmt.Sprintf("Status subresource watching logs since %s: ", patchingTimestamp))
		By("changing the Watcher spec.field to status")
		Expect(updateWatcherSpecField(ctx, controlPlaneClient, watcherCrName)).
			Should(Succeed())

		By("changing the status sub-resource of the remote KymaCR")
		Eventually(updateRemoteKymaStatusSubresource).
			WithContext(ctx).
			WithArguments(runtimeClient, remoteNamespace).
			Should(Succeed())
		By("verifying new reconciliation got triggered for corresponding KymaCR on KCP")
		Eventually(checkKLMLogs).
			WithContext(ctx).
			WithArguments(incomingRequestMsg, controlPlaneRESTConfig, runtimeRESTConfig, controlPlaneClient,
				runtimeClient, patchingTimestamp).
			Should(Succeed())
	})
})

func changeRemoteKymaChannel(ctx context.Context, kymaNamespace, channel string, k8sClient client.Client) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx,
		client.ObjectKey{Name: defaultRemoteKymaName, Namespace: kymaNamespace},
		kyma); err != nil {
		return err
	}

	kyma.Spec.Channel = channel

	return k8sClient.Update(ctx, kyma)
}

func checkKLMLogs(ctx context.Context,
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

	GinkgoWriter.Printf("KLM Logs:  %s\n", logs)
	if strings.Contains(logs, logMsg) {
		return nil
	}

	watcherLogs, err := getPodLogs(ctx, runtimeConfig,
		runtimeClient, remoteNamespace, watcher.SkrResourceName, watcherPodContainer, logsSince)
	if err != nil {
		return err
	}
	GinkgoWriter.Printf("watcher Logs:  %s\n", watcherLogs)
	return errLogNotFound
}

func getPodLogs(ctx context.Context,
	config *rest.Config,
	k8sClient client.Client,
	namespace, podPrefix, container string,
	logsSince *apimetav1.Time,
) (string, error) {
	pod := &apicorev1.Pod{}
	podList := &apicorev1.PodList{}
	if err := k8sClient.List(ctx, podList, &client.ListOptions{Namespace: namespace}); err != nil {
		return "", err
	}

	for _, p := range podList.Items {
		p := p
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
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apicorev1.PodLogOptions{
		Container: container,
		SinceTime: logsSince,
	})
	GinkgoWriter.Printf("Request: %#v", req)
	GinkgoWriter.Printf("Request URL: %s", req.URL())
	podLogs, err := req.Stream(ctx)
	if err != nil {
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
	watcherDeployment := &apiappsv1.Deployment{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      watcherName,
			Namespace: watcherNamespace,
		},
	}
	return k8sClient.Delete(ctx, watcherDeployment)
}

func checkWatcherDeploymentReady(ctx context.Context,
	deploymentName, deploymentNamespace string, k8sClient client.Client,
) error {
	watcherDeployment := &apiappsv1.Deployment{}
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
	certificateSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
		},
	}
	return k8sClient.Delete(ctx, certificateSecret)
}

func checkCertificateSecretExists(ctx context.Context,
	secretName, secretNamespace string, k8sClient client.Client,
) error {
	certificateSecret := &apicorev1.Secret{}
	return k8sClient.Get(ctx,
		client.ObjectKey{Name: secretName, Namespace: secretNamespace},
		certificateSecret,
	)
}

func updateRemoteKymaStatusSubresource(k8sClient client.Client, kymaNamespace string) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: defaultRemoteKymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return fmt.Errorf("failed to get Kyma %w", err)
	}

	kyma.Status.State = shared.StateWarning
	kyma.Status.LastOperation = shared.LastOperation{
		Operation:      "Updated Kyma Status subresource for test",
		LastUpdateTime: apimetav1.NewTime(time.Now()),
	}
	kyma.ManagedFields = nil
	if err := k8sClient.Status().Update(ctx, kyma); err != nil {
		return fmt.Errorf("kyma status subresource could not be updated: %w", err)
	}

	return nil
}

func updateWatcherSpecField(ctx context.Context, k8sClient client.Client, name string) error {
	watcherCR := &v1beta2.Watcher{}
	err := k8sClient.Get(ctx,
		client.ObjectKey{Name: name, Namespace: controlPlaneNamespace},
		watcherCR)
	if err != nil {
		return fmt.Errorf("failed to get Kyma %w", err)
	}
	watcherCR.Spec.Field = v1beta2.StatusField
	if err = k8sClient.Update(ctx, watcherCR); err != nil {
		return fmt.Errorf("failed to update watcher spec.field: %w", err)
	}
	return nil
}
