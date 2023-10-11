package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	watcherPodContainer = "server"

	KLMPodPrefix    = "klm-controller-manager"
	KLMPodContainer = "manager"

	defaultRuntimeNamespace = "kyma-system"
	controlPlaneNamespace   = "kcp-system"

	watcherCrName = "kyma-watcher"
)

var (
	errPodNotFound               = errors.New("could not find pod")
	errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")
	errLogNotFound               = errors.New("logMsg was not found in log")
)

var _ = Describe("Enqueue Event from Watcher", Ordered, func() {
	kyma := testutils.NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)
	incomingRequestMsg := fmt.Sprintf("event received from SKR, adding %s/%s to queue",
		kyma.GetNamespace(), kyma.GetName())

	BeforeAll(func() {
		// make sure we can list Kymas to ensure CRDs have been installed
		err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
		Expect(meta.IsNoMatchError(err)).To(BeFalse())
	})

	It("Should create empty Kyma CR on remote cluster", func() {
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
		Eventually(controlPlaneClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("verifying kyma is ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
		By("verifying remote kyma is ready")
		Eventually(CheckRemoteKymaCR).
			WithContext(ctx).
			WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
			Should(Succeed())
	})

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
		timeNow := &metav1.Time{Time: time.Now()}
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
		patchingTimestamp := &metav1.Time{Time: time.Now()}
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

	It("Should delete Kyma CR on remote cluster", func() {
		Eventually(deleteKymaCR).
			WithContext(ctx).
			WithArguments(kyma, controlPlaneClient).
			Should(Succeed())

		Eventually(DeleteKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())

		Eventually(checkRemoteKymaCRDeleted).
			WithContext(ctx).
			WithArguments(remoteNamespace, runtimeClient).
			Should(Succeed())
	})
})

func deleteKymaCR(ctx context.Context, kyma *v1beta2.Kyma, k8sClient client.Client) error {
	if err := k8sClient.Delete(ctx, kyma); util.IsNotFound(err) {
		return nil
	}

	if err := k8sClient.Get(ctx,
		client.ObjectKey{Name: kyma.GetName(), Namespace: kyma.GetNamespace()}, kyma); util.IsNotFound(err) {
		return nil
	}

	if !kyma.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(kyma, v1beta2.PurgeFinalizer) {
			controllerutil.RemoveFinalizer(kyma, v1beta2.PurgeFinalizer)
			if err := k8sClient.Update(ctx, kyma); err != nil {
				return err
			}
		}
	}
	return errKymaNotDeleted
}

func checkRemoteKymaCRDeleted(ctx context.Context,
	kymaNamespace string, k8sClient client.Client,
) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: defaultRemoteKymaName, Namespace: kymaNamespace}, kyma)
	if util.IsNotFound(err) {
		return nil
	}
	return err
}

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
	logsSince *metav1.Time,
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
		runtimeClient, defaultRuntimeNamespace, watcher.SkrResourceName, watcherPodContainer, logsSince)
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
	logsSince *metav1.Time,
) (string, error) {
	pod := &corev1.Pod{}
	podList := &corev1.PodList{}
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
	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
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

func updateRemoteKymaStatusSubresource(k8sClient client.Client, kymaNamespace string) error {
	kyma := &v1beta2.Kyma{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: defaultRemoteKymaName, Namespace: kymaNamespace}, kyma)
	if err != nil {
		return fmt.Errorf("failed to get Kyma %w", err)
	}

	kyma.Status.State = v1beta2.StateWarning
	kyma.Status.LastOperation = v1beta2.LastOperation{
		Operation:      "Updated Kyma Status subresource for test",
		LastUpdateTime: metav1.NewTime(time.Now()),
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
