package withwatcher_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	. "github.com/kyma-project/lifecycle-manager/operator/internal/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	webhookConfigNameTpl           = "%s-webhook"
	servicePathTpl                 = "/validate/%s"
	specSubresources               = "*"
	statusSubresources             = "*/status"
	expectedWebhookNamePartsLength = 4
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	kyma := NewTestKyma("kyma-remote-sync")
	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    metav1.NamespaceDefault,
		NoModuleCopy: true,
	}
	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	registerDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma)

	It("kyma reconciler installs watcher helm chart with correct webhook config", func() {
		By("waiting for skr webhook condition to be ready")
		Eventually(IsSKRWebhookConditionReasonReady(suiteCtx, controlPlaneClient, kyma.Name),
			Timeout, Interval).Should(BeTrue())
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		By("waiting for webhook configuration to be deployed in the remote cluster")
		Eventually(runtimeClient.Get(suiteCtx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(webhookConfigNameTpl, client.ObjectKeyFromObject(kyma)),
		}, webhookConfig), Timeout, Interval).Should(Succeed())
		Expect(isWebhookConfigured(watcherCrForKyma, webhookConfig, kyma.Name)).To(Succeed())
	})

	It("kyma reconciler installs watcher helm chart with correct webhook config when watcher specs are updated",
		func() {
			labelKey := "new-key"
			labelValue := "new-value"
			watcherCrForKyma.Spec.LabelsToWatch[labelKey] = labelValue
			Expect(controlPlaneClient.Update(suiteCtx, watcherCrForKyma)).To(Succeed())
			By("waiting for watcher CR labelsToWatch to be updated")
			Eventually(isWatcherCrLabelUpdated(client.ObjectKeyFromObject(watcherCrForKyma),
				labelKey, labelValue), Timeout, Interval).Should(BeTrue())
			By("updating kyma channel to trigger its reconciliation")
			kyma.Spec.Channel = v1alpha1.ChannelFast
			Expect(controlPlaneClient.Update(suiteCtx, kyma)).To(Succeed())
			By("waiting for the kyma update event to be processed")
			time.Sleep(2 * time.Second)
			By("waiting for skr webhook condition to be ready")
			Eventually(IsSKRWebhookConditionReasonReady(suiteCtx, controlPlaneClient, kyma.Name),
				Timeout, Interval).Should(BeTrue())
			webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(runtimeClient.Get(suiteCtx, client.ObjectKey{
				Namespace: metav1.NamespaceDefault,
				Name:      deploy.ResolveSKRChartResourceName(webhookConfigNameTpl, client.ObjectKeyFromObject(kyma)),
			}, webhookConfig)).To(Succeed())
			Expect(isWebhookConfigured(watcherCrForKyma, webhookConfig, kyma.Name)).To(Succeed())
		})

	It("webhook manager removes watcher helm chart from SKR cluster when kyma is deleted", func() {
		Expect(controlPlaneClient.Delete(suiteCtx, kyma)).To(Succeed())
		Eventually(getSkrChartDeployment(suiteCtx, runtimeClient, client.ObjectKeyFromObject(kyma)), Timeout, Interval).
			ShouldNot(Succeed())
		Eventually(isKymaCrDeletionFinished(client.ObjectKeyFromObject(kyma)), Timeout, Interval).
			Should(BeTrue())
	})
})

func registerDefaultLifecycleForKymaWithWatcher(kyma *v1alpha1.Kyma, watcher *v1alpha1.Watcher) {
	BeforeAll(func() {
		By("Creating watcher CR")
		Expect(controlPlaneClient.Create(suiteCtx, watcher)).To(Succeed())
		By("Creating kyma CR")
		Expect(controlPlaneClient.Create(suiteCtx, kyma)).To(Succeed())
	})

	AfterAll(func() {
		By("Deleting watcher CR")
		Expect(controlPlaneClient.Delete(suiteCtx, watcher)).To(Succeed())
		By("Ensuring watcher CR is properly deleted")
		Eventually(isWatcherCrDeletionFinished(client.ObjectKeyFromObject(watcher)), Timeout, Interval).
			Should(BeTrue())
		// By("Deleting kyma CR")
		// Expect(controlPlaneClient.Delete(suiteCtx, kyma)).Should(Succeed())
		// By("Ensuring kyma CR is properly deleted")
		// Eventually(isKymaCrDeletionFinished(client.ObjectKeyFromObject(kyma)), Timeout, Interval).
		// 	Should(BeTrue())
	})

	BeforeEach(func() {
		By("deleting rendered manifests")
		Expect(os.RemoveAll(filepath.Join(webhookChartPath, "manifest"))).To(Succeed())
		By("asserting only one kyma CR exists")
		kcpKymas := &v1alpha1.KymaList{}
		Expect(controlPlaneClient.List(suiteCtx, kcpKymas)).To(Succeed())
		Expect(kcpKymas.Items).NotTo(BeEmpty())
		Expect(len(kcpKymas.Items)).To(Equal(1))
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(suiteCtx, client.ObjectKeyFromObject(kyma), kyma)).To(Succeed())
		By("get latest watcher CR")
		Expect(controlPlaneClient.Get(suiteCtx, client.ObjectKeyFromObject(watcher), watcher)).
			To(Succeed())
	})
	AfterEach(func() {
		By("deleting rendered manifests")
		Expect(os.RemoveAll(filepath.Join(webhookChartPath, "manifest"))).To(Succeed())
	})
}
func isWatcherCrLabelUpdated(watcherObjKey client.ObjectKey, labelKey, expectedLabelValue string) func() bool {
	return func() bool {
		watcher := &v1alpha1.Watcher{}
		err := controlPlaneClient.Get(suiteCtx, watcherObjKey, watcher)
		if err != nil {
			return false
		}
		labelValue, ok := watcher.Spec.LabelsToWatch[labelKey]
		if !ok {
			return false
		}
		return expectedLabelValue == labelValue
	}
}

func isKymaCrDeletionFinished(kymaObjKey client.ObjectKey) func() bool {
	return func() bool {
		err := controlPlaneClient.Get(suiteCtx, kymaObjKey, &v1alpha1.Kyma{})
		return apierrors.IsNotFound(err)
	}
}

func IsSKRWebhookConditionReasonReady(ctx context.Context, kcpClient client.Client, kymaName string) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaName)
		if err != nil {
			return false
		}
		for _, kymaCondition := range kymaFromCluster.Status.Conditions {
			webhookChartIsInstalled := kymaCondition.Type == string(v1alpha1.ConditionTypeReady) &&
				kymaCondition.Reason == string(v1alpha1.ConditionReasonSKRWebhookIsReady) &&
				kymaCondition.Status == metav1.ConditionTrue
			if webhookChartIsInstalled {
				return true
			}
		}
		return false
	}
}

func getSkrChartDeployment(ctx context.Context, skrClient client.Client, kymaObjKey client.ObjectKey) func() error {
	return func() error {
		return skrClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.DeploymentNameTpl, kymaObjKey),
		}, &appsv1.Deployment{})
	}
}

func isWebhookConfigured(watcher *v1alpha1.Watcher, webhookConfig *admissionv1.ValidatingWebhookConfiguration,
	kymaName string,
) error {
	if len(webhookConfig.Webhooks) < 1 {
		return fmt.Errorf("Expected at least one webhook configured: (kyma=%s, webconfig=%s)", kymaName, webhookConfig.Name)
	}
	idx := lookupWebhookConfigForCR(webhookConfig.Webhooks, watcher)
	if idx == -1 {
		return fmt.Errorf("Webhook config matching Watcher CR not found: (kyma=%s, webconfig=%s)", kymaName, webhookConfig.Name)
	}
	return verifyWebhookConfig(webhookConfig.Webhooks[idx], watcher)
}

func lookupWebhookConfigForCR(webhooks []admissionv1.ValidatingWebhook, watcher *v1alpha1.Watcher) int {
	cfgIdx := -1
	for idx, webhook := range webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) == 0 {
			continue
		}
		moduleName := webhookNameParts[0]
		objModuleName := watcher.GetModuleName()
		if moduleName == objModuleName {
			return idx
		}
	}
	return cfgIdx
}

func verifyWebhookConfig(
	webhook admissionv1.ValidatingWebhook,
	watcherCR *v1alpha1.Watcher,
) error {
	// TODO(khlifi411): refactor: return errors instead of using gomega's Expect
	webhookNameParts := strings.Split(webhook.Name, ".")
	Expect(len(webhookNameParts)).To(Equal(expectedWebhookNamePartsLength))
	moduleName := webhookNameParts[0]
	expectedModuleName, exists := watcherCR.Labels[v1alpha1.ManagedBylabel]
	Expect(exists).To(BeTrue())
	Expect(moduleName).To(Equal(expectedModuleName))
	Expect(*webhook.ClientConfig.Service.Path).To(Equal(fmt.Sprintf(servicePathTpl, moduleName)))
	Expect(reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watcherCR.Spec.LabelsToWatch)).To(BeTrue())
	if watcherCR.Spec.Field == v1alpha1.StatusField {
		Expect(webhook.Rules[0].Resources[0]).To(Equal(statusSubresources))
	}
	if watcherCR.Spec.Field == v1alpha1.SpecField {
		Expect(webhook.Rules[0].Resources[0]).To(Equal(specSubresources))
	}
	return nil
}
