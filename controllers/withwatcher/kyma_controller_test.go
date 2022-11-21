package withwatcher_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/pkg/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	servicePathTpl                 = "/validate/%s"
	specSubresources               = "*"
	statusSubresources             = "*/status"
	expectedWebhookNamePartsLength = 4
	dummyCaBundleData              = "amVsbHlmaXNoLXdhcy1oZXJl"
)

var (
	ErrExpectedAtLeastOneWebhook       = errors.New("expected at least one webhook configured")
	ErrWebhookConfigForWatcherNotFound = errors.New("webhook config matching Watcher CR not found")
	ErrWebhookNamePartsNumberMismatch  = errors.New("webhook name dot separated parts number mismatch")
	ErrManagedByLabelNotFound          = errors.New("managed-by label not found")
	ErrModuleNameMismatch              = errors.New("module name mismatch")
	ErrSvcPathMismatch                 = errors.New("service path mismatch")
	ErrWatchLabelsMismatch             = errors.New("watch labels mismatch")
	ErrStatusSubResourcesMismatch      = errors.New("status sub-resources mismatch")
	ErrSpecSubResourcesMismatch        = errors.New("spec sub-resources mismatch")
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, Serial, func() {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	kyma := NewTestKyma("kyma-remote-sync")
	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    metav1.NamespaceDefault,
		NoModuleCopy: true,
	}
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	registerDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma)

	It("kyma reconciler installs watcher helm chart with correct webhook config", func() {
		By("waiting for skr webhook condition to be ready")
		Eventually(IsSKRWebhookConditionReasonReady(suiteCtx, controlPlaneClient, kymaObjKey),
			Timeout, Interval).Should(BeTrue())
		By("waiting for webhook configuration to be deployed in the remote cluster")
		Eventually(runtimeClient.Get(suiteCtx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.WebhookConfigNameTpl, kymaObjKey),
		}, webhookConfig), Timeout, Interval).Should(Succeed())
		Expect(isWebhookConfigured(watcherCrForKyma, webhookConfig, kyma.Name)).To(Succeed())
	})

	It("kyma reconciler re-installs watcher helm chart when webhook CA bundle is not consistent", func() {
		By("updating webhook config with corrupt CA bundle data")
		Expect(webhookConfig.Webhooks).NotTo(BeEmpty())
		webhookConfig.Webhooks[0].ClientConfig.CABundle = []byte(dummyCaBundleData)
		By("updating kyma channel to trigger its reconciliation")
		kyma.Spec.Channel = v1alpha1.ChannelRapid
		Expect(controlPlaneClient.Update(suiteCtx, kyma)).To(Succeed())
		By("waiting for the kyma update event to be processed")
		time.Sleep(2 * time.Second)
		By("waiting for skr webhook condition to be ready")
		Eventually(IsSKRWebhookConditionReasonReady(suiteCtx, controlPlaneClient, kymaObjKey),
			Timeout, Interval).Should(BeTrue())
		Expect(deploy.CheckWebhookCABundleConsistency(suiteCtx, runtimeClient, kymaObjKey)).To(Succeed())
	})

	It("Watcher helm chart caching works as expected", func() {
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
		Eventually(IsSKRWebhookConditionReasonReady(suiteCtx, controlPlaneClient, kymaObjKey),
			Timeout, Interval).Should(BeTrue())
		Expect(runtimeClient.Get(suiteCtx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.WebhookConfigNameTpl, kymaObjKey),
		}, webhookConfig)).To(Succeed())
		Expect(isWebhookConfigured(watcherCrForKyma, webhookConfig, kyma.Name)).To(Succeed())

		caBundleValueBeforeUpdate := webhookConfig.Webhooks[0].ClientConfig.CABundle
		By("updating kyma channel to trigger its reconciliation")
		kyma.Spec.Channel = v1alpha1.ChannelRapid
		Expect(controlPlaneClient.Update(suiteCtx, kyma)).To(Succeed())
		By("waiting for the kyma update event to be processed")
		time.Sleep(2 * time.Second)
		By("waiting for skr webhook condition to be ready")
		Eventually(IsSKRWebhookConditionReasonReady(suiteCtx, controlPlaneClient, kymaObjKey),
			Timeout, Interval).Should(BeTrue())
		Expect(runtimeClient.Get(suiteCtx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.WebhookConfigNameTpl, kymaObjKey),
		}, webhookConfig)).To(Succeed())
		Expect(string(webhookConfig.Webhooks[0].ClientConfig.CABundle)).To(Equal(string(caBundleValueBeforeUpdate)))
	})

	It("webhook manager removes watcher helm chart from SKR cluster when kyma is deleted", func() {
		Expect(controlPlaneClient.Delete(suiteCtx, kyma)).To(Succeed())
		Eventually(getSkrChartDeployment(suiteCtx, runtimeClient, kymaObjKey), Timeout, Interval).
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

func IsSKRWebhookConditionReasonReady(ctx context.Context, kcpClient client.Client,
	kymaObjKey client.ObjectKey,
) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaObjKey.Name, kymaObjKey.Namespace)
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
		return fmt.Errorf("%w: (kyma=%s, webconfig=%s)", ErrExpectedAtLeastOneWebhook,
			kymaName, webhookConfig.Name)
	}
	idx := lookupWebhookConfigForCR(webhookConfig.Webhooks, watcher)
	if idx == -1 {
		return fmt.Errorf("%w: (kyma=%s, webconfig=%s)", ErrWebhookConfigForWatcherNotFound,
			kymaName, webhookConfig.Name)
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
	webhookNameParts := strings.Split(webhook.Name, ".")
	if len(webhookNameParts) != expectedWebhookNamePartsLength {
		return fmt.Errorf("%w: (webhook=%s)", ErrWebhookNamePartsNumberMismatch, webhook.Name)
	}
	moduleName := webhookNameParts[0]
	expectedModuleName, exists := watcherCR.Labels[v1alpha1.ManagedBy]
	if !exists {
		return fmt.Errorf("%w: (labels=%v)", ErrManagedByLabelNotFound, watcherCR.Labels)
	}
	if moduleName != expectedModuleName {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrModuleNameMismatch,
			expectedModuleName, moduleName)
	}
	expectedSvcPath := fmt.Sprintf(servicePathTpl, moduleName)
	if *webhook.ClientConfig.Service.Path != expectedSvcPath {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrSvcPathMismatch,
			expectedSvcPath, *webhook.ClientConfig.Service.Path)
	}
	if !reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watcherCR.Spec.LabelsToWatch) {
		return fmt.Errorf("%w: (expected=%v, got=%v)", ErrWatchLabelsMismatch,
			watcherCR.Spec.LabelsToWatch, webhook.ObjectSelector.MatchLabels)
	}
	if watcherCR.Spec.Field == v1alpha1.StatusField && webhook.Rules[0].Resources[0] != statusSubresources {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrStatusSubResourcesMismatch,
			statusSubresources, webhook.Rules[0].Resources[0])
	}
	if watcherCR.Spec.Field == v1alpha1.SpecField && webhook.Rules[0].Resources[0] != specSubresources {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrSpecSubResourcesMismatch,
			specSubresources, webhook.Rules[0].Resources[0])
	}
	return nil
}
