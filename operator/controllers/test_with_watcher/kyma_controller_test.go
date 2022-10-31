package controllers_with_watcher_test

import (
	"errors"
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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespace                      = "default"
	servicePathTpl                 = "/validate/%s"
	specSubresources               = "*"
	statusSubresources             = "*/status"
	expectedWebhookNamePartsLength = 4
)

var (
	ErrNoWebhooksEntry        = errors.New("validatingWebhookConfiguration webhooks entry not exists")
	ErrCaCertNotExitsInSecret = errors.New("ca cert not exists in secrets")
)

var _ = Describe("Test Kyma reconcile", Ordered, func() {
	kyma := testhelper.NewTestKyma("kyma")
	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)

	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    namespace,
		NoModuleCopy: true,
	}
	RegisterDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma)

	DescribeTable("watcher enabled",
		func(givenCondition func() error, expectBehaviors ...func() error) {
			Eventually(givenCondition, timeout, interval).Should(Succeed())
			for _, expectBehavior := range expectBehaviors {
				Eventually(expectBehavior, timeout, interval).Should(Succeed())
			}
		},
		Entry("When Kyma CR deployed, expect skr webhook resources deployed correctly in remote cluster",
			testhelper.NoCondition(),
			expectSkrWebhookResourcesCorrectlyDeployed(watcherCrForKyma),
			expectKymaInState(kyma.GetName(), v1alpha1.StateReady)),
		Entry("When Kyma CR removed, expect skr webhook resources remoted",
			deleteKyma(kyma),
			expectSkrWebhookResourcesRemoved()),
	)
})

func expectKymaInState(kymaName string, expectState v1alpha1.State) func() error {
	return func() error {
		return testhelper.KymaInState(suiteCtx, controlPlaneClient, kymaName, expectState)
	}
}

func deleteKyma(kyma *v1alpha1.Kyma) func() error {
	return func() error {
		return testhelper.DeleteKyma(suiteCtx, controlPlaneClient, kyma)
	}
}

func expectSkrWebhookResourcesRemoved() func() error {
	return func() error {
		if _, err := getSkrWebhookDeployment(); err != nil {
			return client.IgnoreNotFound(err)
		}

		if _, err := getSkrWebhookSecret(); err != nil {
			return client.IgnoreNotFound(err)
		}

		if _, err := getValidatingWebhookConfiguration(); err != nil {
			return client.IgnoreNotFound(err)
		}
		return nil
	}
}

func expectSkrWebhookResourcesCorrectlyDeployed(watcher *v1alpha1.Watcher) func() error {
	return func() error {
		if _, err := getSkrWebhookDeployment(); err != nil {
			return err
		}
		secret, err := getSkrWebhookSecret()
		if err != nil {
			return err
		}
		caCert, exists := secret.Data["ca.crt"]
		if !exists {
			return ErrCaCertNotExitsInSecret
		}
		return expectValidatingWebhookConfigurationCorrectlyConfigured(watcher, caCert)
	}
}

func getSkrWebhookSecret() (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	err := runtimeClient.Get(suiteCtx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      deploy.ResolveSKRChartResourceName(),
	}, secret)
	return secret, err
}

func getSkrWebhookDeployment() (*appsv1.Deployment, error) {
	skrWebhookDeploy := &appsv1.Deployment{}
	err := runtimeClient.Get(suiteCtx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      deploy.ResolveSKRChartResourceName(),
	}, skrWebhookDeploy)
	return skrWebhookDeploy, err
}

func expectValidatingWebhookConfigurationCorrectlyConfigured(watcher *v1alpha1.Watcher, caCert []byte) error {
	webhookConfig, err := getValidatingWebhookConfiguration()
	if err != nil {
		return err
	}
	return verifyWebhookConfig(webhookConfig, watcher, caCert)
}

func getValidatingWebhookConfiguration() (*admissionv1.ValidatingWebhookConfiguration, error) {
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err := runtimeClient.Get(suiteCtx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      deploy.ResolveSKRChartResourceName(),
	}, webhookConfig)
	if err != nil {
		return nil, err
	}
	return webhookConfig, nil
}

func buildWebhookLookupMap(webhooks []admissionv1.ValidatingWebhook) map[string]admissionv1.ValidatingWebhook {
	lookupMap := make(map[string]admissionv1.ValidatingWebhook)
	for _, webhook := range webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) == 0 {
			continue
		}
		moduleName := webhookNameParts[0]
		lookupMap[moduleName] = webhook
	}
	return lookupMap
}

func verifyWebhookConfig(webhookConfig *admissionv1.ValidatingWebhookConfiguration, watcherCR *v1alpha1.Watcher, caCert []byte) error {
	lookupMap := buildWebhookLookupMap(webhookConfig.Webhooks)
	webhook, exist := lookupMap[watcherCR.GetModuleName()]
	if !exist {
		return ErrNoWebhooksEntry
	}

	webhookNameParts := strings.Split(webhook.Name, ".")
	Expect(len(webhookNameParts)).To(Equal(expectedWebhookNamePartsLength))

	moduleName := webhookNameParts[0]
	expectedModuleName, exists := watcherCR.Labels[v1alpha1.ManagedBylabel]
	Expect(exists).To(BeTrue())
	Expect(moduleName).To(Equal(expectedModuleName))
	Expect(*webhook.ClientConfig.Service.Path).To(Equal(fmt.Sprintf(servicePathTpl, moduleName)))
	Expect(reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watcherCR.Spec.LabelsToWatch)).To(BeTrue())
	Expect(webhook.ClientConfig.CABundle).To(Equal(caCert))

	if watcherCR.Spec.Field == v1alpha1.StatusField {
		Expect(webhook.Rules[0].Resources[0]).To(Equal(statusSubresources))
	}

	if watcherCR.Spec.Field == v1alpha1.SpecField {
		Expect(webhook.Rules[0].Resources[0]).To(Equal(specSubresources))
	}
	return nil
}

func RegisterDefaultLifecycleForKymaWithWatcher(kyma *v1alpha1.Kyma, watcher *v1alpha1.Watcher) {
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(suiteCtx, watcher)).Should(Succeed())
		Expect(controlPlaneClient.Create(suiteCtx, kyma)).Should(Succeed())
		testhelper.DeployModuleTemplates(suiteCtx, controlPlaneClient, kyma)
	})

	AfterAll(func() {
		err := controlPlaneClient.Delete(suiteCtx, watcher)
		Expect(client.IgnoreNotFound(err)).Should(Succeed())
		testhelper.DeleteModuleTemplates(suiteCtx, controlPlaneClient, kyma)
		err = controlPlaneClient.Delete(suiteCtx, kyma)
		Expect(client.IgnoreNotFound(err)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(suiteCtx, client.ObjectKey{
			Name: kyma.Name, Namespace: metav1.NamespaceDefault,
		}, kyma)).Should(Succeed())
	})
}
