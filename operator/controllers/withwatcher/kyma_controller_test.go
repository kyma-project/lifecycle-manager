package withwatcher_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers/testhelper"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace                      = "default"
	webhookConfigNameTpl           = "%s-webhook"
	servicePathTpl                 = "/validate/%s"
	specSubresources               = "*"
	statusSubresources             = "*/status"
	expectedWebhookNamePartsLength = 4
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	kyma := testhelper.NewTestKyma("kyma-remote-sync")
	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)

	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    namespace,
		NoModuleCopy: true,
	}
	RegisterDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma)

	It("kyma reconciler installs watcher helm chart with correct webhook config", func() {
		webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
		Eventually(isWebhookDeployed(suiteCtx, runtimeClient, webhookConfig), timeout, interval).
			Should(Succeed())
		Expect(isWebhookConfigured(watcherCrForKyma, webhookConfig)).To(BeTrue())
		Eventually(testhelper.IsKymaInState(suiteCtx, controlPlaneClient, kyma.GetName(), v1alpha1.StateReady),
			timeout, interval).Should(BeTrue())
	})

	It("webhook manager removes watcher helm chart from SKR cluster when kyma is deleted", func() {
		latestKyma := &v1alpha1.Kyma{}
		Expect(controlPlaneClient.Get(suiteCtx, client.ObjectKeyFromObject(kyma), latestKyma)).To(Succeed())
		Expect(controlPlaneClient.Delete(suiteCtx, latestKyma)).To(Succeed())
		Eventually(getSkrChartDeployment(suiteCtx, runtimeClient), timeout, interval).Should(Succeed())
	})
})

func getSkrChartDeployment(ctx context.Context, skrClient client.Client) func() error {
	return func() error {
		return skrClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.DeploymentNameTpl),
		}, &appsv1.Deployment{})
	}
}

func isWebhookDeployed(ctx context.Context, skrClient client.Client,
	webhookConfig *admissionv1.ValidatingWebhookConfiguration,
) func() error {
	return func() error {
		return skrClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(webhookConfigNameTpl),
		}, webhookConfig)
	}
}

func isWebhookConfigured(watcher *v1alpha1.Watcher, webhookConfig *admissionv1.ValidatingWebhookConfiguration) bool {
	if len(webhookConfig.Webhooks) < 1 {
		return false
	}
	idx := lookupWebhookConfigForCR(webhookConfig.Webhooks, watcher)
	if idx != -1 {
		return verifyWebhookConfig(webhookConfig.Webhooks[idx], watcher)
	}
	return false
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
) bool {
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
	return true
}

func RegisterDefaultLifecycleForKymaWithWatcher(kyma *v1alpha1.Kyma, watcher *v1alpha1.Watcher) {
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(suiteCtx, watcher)).To(Succeed())
		Expect(controlPlaneClient.Create(suiteCtx, kyma)).Should(Succeed())
		testhelper.DeployModuleTemplates(suiteCtx, controlPlaneClient, kyma)
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(suiteCtx, watcher)).To(Succeed())
		testhelper.DeleteModuleTemplates(suiteCtx, controlPlaneClient, kyma)
		Expect(controlPlaneClient.Delete(suiteCtx, kyma)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(suiteCtx, client.ObjectKey{
			Name: kyma.Name, Namespace: metav1.NamespaceDefault,
		}, kyma)).Should(Succeed())
	})
}
