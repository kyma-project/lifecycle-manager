package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	apiappsv1 "k8s.io/api/apps/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	watcherCrName = "klm-kyma-watcher"
)

var errWatcherDeploymentNotReady = errors.New("watcher Deployment is not ready")

var _ = Describe("Enqueue Event from Watcher", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)
	incomingRequestMsg := fmt.Sprintf("event received from SKR, adding %s/%s to queue",
		kyma.GetNamespace(), kyma.GetName())

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)
	skrNamespacedSecretName := k8stypes.NamespacedName{
		Name:      watcher.SkrTLSName,
		Namespace: remoteNamespace,
	}

	Context("Given SKR Cluster with TLS Secret", func() {
		It("When Runtime Watcher deployment is ready", func() {
			Eventually(checkWatcherDeploymentReady).
				WithContext(ctx).
				WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
				Should(Succeed())

			By("And Runtime Watcher deployment is deleted")
			Eventually(deleteWatcherDeployment).
				WithContext(ctx).
				WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
				Should(Succeed())
		})

		It("Then Runtime Watcher deployment is ready again", func() {
			Eventually(checkWatcherDeploymentReady).
				WithContext(ctx).
				WithArguments(watcher.SkrResourceName, remoteNamespace, runtimeClient).
				Should(Succeed())
		})

		It("When TLS Secret is deleted on SKR Cluster", func() {
			Eventually(CertificateSecretExists).
				WithContext(ctx).
				WithArguments(skrNamespacedSecretName, runtimeClient).
				Should(Succeed())
			Eventually(DeleteCertificateSecret).
				WithContext(ctx).
				WithArguments(skrNamespacedSecretName, runtimeClient).
				Should(Succeed())
		})

		It("Then TLS Secret is recreated", func() {
			Eventually(CertificateSecretExists).
				WithContext(ctx).
				WithArguments(skrNamespacedSecretName, runtimeClient).
				Should(Succeed())
		})

		timeNow := &apimetav1.Time{Time: time.Now()}
		It("When spec of SKR Kyma CR is changed", func() {
			GinkgoWriter.Println(fmt.Sprintf("Spec watching logs since %s: ", timeNow))
			switchedChannel := "fast"
			Eventually(changeRemoteKymaChannel).
				WithContext(ctx).
				WithArguments(remoteNamespace, switchedChannel, runtimeClient).
				Should(Succeed())
		})

		It("Then new reconciliation gets triggered for KCP Kyma CR", func() {
			Eventually(CheckKLMLogs).
				WithContext(ctx).
				WithArguments(incomingRequestMsg, controlPlaneRESTConfig, runtimeRESTConfig,
					controlPlaneClient, runtimeClient, timeNow).
				Should(Succeed())
		})

		time.Sleep(1 * time.Second)
		patchingTimestamp := &apimetav1.Time{Time: time.Now()}
		GinkgoWriter.Println(fmt.Sprintf("Status subresource watching logs since %s: ", patchingTimestamp))
		It("When Runtime Watcher spec field is changed to status", func() {
			Expect(updateWatcherSpecField(ctx, controlPlaneClient, watcherCrName)).
				Should(Succeed())

			By("And SKR Kyma CR Status is updated")
			Eventually(updateRemoteKymaStatusSubresource).
				WithContext(ctx).
				WithArguments(runtimeClient, remoteNamespace).
				Should(Succeed())
		})

		It("Then new reconciliation gets triggered for KCP Kyma CR", func() {
			Eventually(CheckKLMLogs).
				WithContext(ctx).
				WithArguments(incomingRequestMsg, controlPlaneRESTConfig, runtimeRESTConfig,
					controlPlaneClient, runtimeClient, patchingTimestamp).
				Should(Succeed())
		})
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
