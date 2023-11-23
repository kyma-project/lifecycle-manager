package withwatcher_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiappsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	servicePathTpl                 = "/validate/%s"
	expectedWebhookNamePartsLength = 5
)

var (
	ErrExpectedAtLeastOneWebhook       = errors.New("expected at least one webhook configured")
	ErrWebhookConfigForWatcherNotFound = errors.New("webhook config matching Watcher CR not found")
	ErrWebhookNamePartsNumberMismatch  = errors.New("webhook name dot separated parts number mismatch")
	ErrManagedByLabelNotFound          = errors.New("managed-by label not found")
	ErrWebhookCfgNameMismatch          = errors.New("webhook config name mismatch")
	ErrSvcPathMismatch                 = errors.New("service path mismatch")
	ErrWatchLabelsMismatch             = errors.New("watch labels mismatch")
	ErrResourcesMismatch               = errors.New("resources mismatch")
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	kyma := NewTestKyma("kyma-remote-sync")

	watcherCrForKyma := createWatcherCR("skr-webhook-manager", true)
	issuer := NewTestIssuer(istioSystemNs)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	tlsSecret := createTLSSecret(kymaObjKey)
	caCertificate := createCaCertificate()

	registerDefaultLifecycleForKymaWithWatcher(kyma, watcherCrForKyma, tlsSecret, issuer, caCertificate)

	It("kyma reconciliation installs watcher with correct webhook config", func() {
		Eventually(latestWebhookIsConfigured(suiteCtx, runtimeClient, watcherCrForKyma,
			kymaObjKey), Timeout, Interval).Should(Succeed())
	})

	It("kyma reconciliation replaces webhook-config when a new watcher is created and deleted", func() {
		secondWatcher := createWatcherCR("second-manager", false)
		By("Creating second watcher CR")
		Expect(controlPlaneClient.Create(suiteCtx, secondWatcher)).To(Succeed())
		Eventually(latestWebhookIsConfigured(suiteCtx, runtimeClient, watcherCrForKyma, kymaObjKey),
			Timeout, Interval).Should(Succeed())
		Eventually(latestWebhookIsConfigured(suiteCtx, runtimeClient, secondWatcher, kymaObjKey),
			Timeout, Interval).Should(Succeed())
		By("Deleting second watcher CR")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(controlPlaneClient, secondWatcher).Should(Succeed())
		By("Ensuring second watcher CR is properly deleted")
		Eventually(isWatcherCrDeletionFinished, Timeout, Interval).WithArguments(secondWatcher).
			Should(BeTrue())
		By("ensuring skr resources are configured for the non-removed watcher CRs")
		Eventually(latestWebhookIsConfigured(suiteCtx, runtimeClient, watcherCrForKyma, kymaObjKey),
			Timeout, Interval).Should(Succeed())
		By("ensuring skr resources are not configured for the removed watcher CR")
		Eventually(latestWebhookIsConfigured(suiteCtx, runtimeClient, secondWatcher, kymaObjKey),
			Timeout, Interval).ShouldNot(Succeed())
	})

	It("SKR chart installation works correctly when watcher config is updated", func() {
		labelKey := "new-key"
		labelValue := "new-value"
		watcherCrForKyma.Spec.LabelsToWatch[labelKey] = labelValue
		Expect(controlPlaneClient.Update(suiteCtx, watcherCrForKyma)).To(Succeed())
		By("waiting for watcher CR labelsToWatch to be updated")
		Eventually(isWatcherCrLabelUpdated(client.ObjectKeyFromObject(watcherCrForKyma),
			labelKey, labelValue), Timeout, Interval).Should(BeTrue())
		Eventually(latestWebhookIsConfigured(suiteCtx, runtimeClient, watcherCrForKyma,
			kymaObjKey), Timeout, Interval).Should(Succeed())
	})

	It("kyma reconciliation removes watcher from SKR cluster when kyma is deleted", func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(suiteCtx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		Eventually(getSkrChartDeployment(suiteCtx, runtimeClient, kymaObjKey), Timeout, Interval).
			ShouldNot(Succeed())
		Eventually(isKymaCrDeletionFinished, Timeout, Interval).
			WithArguments(client.ObjectKeyFromObject(kyma)).Should(BeTrue())
	})
})

func isWatcherCrLabelUpdated(watcherObjKey client.ObjectKey, labelKey, expectedLabelValue string) func() bool {
	return func() bool {
		watcherCR := &v1beta2.Watcher{}
		err := controlPlaneClient.Get(suiteCtx, watcherObjKey, watcherCR)
		if err != nil {
			return false
		}
		labelValue, ok := watcherCR.Spec.LabelsToWatch[labelKey]
		if !ok {
			return false
		}
		return expectedLabelValue == labelValue
	}
}

func isKymaCrDeletionFinished(kymaObjKey client.ObjectKey) bool {
	err := controlPlaneClient.Get(suiteCtx, kymaObjKey, &v1beta2.Kyma{})
	return util.IsNotFound(err)
}

func getSkrChartDeployment(ctx context.Context, skrClient client.Client, kymaObjKey client.ObjectKey) func() error {
	return func() error {
		return skrClient.Get(ctx, client.ObjectKey{
			Namespace: kymaObjKey.Namespace,
			Name:      watcher.SkrResourceName,
		}, &apiappsv1.Deployment{})
	}
}

func latestWebhookIsConfigured(ctx context.Context, skrClient client.Client, watcher *v1beta2.Watcher,
	kymaObjKey client.ObjectKey,
) func() error {
	return func() error {
		webhookCfg, err := getSKRWebhookConfig(ctx, skrClient, kymaObjKey)
		if err != nil {
			return err
		}
		return isWebhookConfigured(watcher, webhookCfg, kymaObjKey.Name)
	}
}

func getSKRWebhookConfig(ctx context.Context, skrClient client.Client,
	kymaObjKey client.ObjectKey,
) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	webhookCfg := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	err := skrClient.Get(ctx, client.ObjectKey{
		Namespace: kymaObjKey.Namespace,
		Name:      watcher.SkrResourceName,
	}, webhookCfg)
	return webhookCfg, err
}

func isWebhookConfigured(watcher *v1beta2.Watcher,
	webhookConfig *admissionregistrationv1.ValidatingWebhookConfiguration,
	kymaName string,
) error {
	if len(webhookConfig.Webhooks) < 1 {
		return fmt.Errorf("%w: (kyma=%s, webconfig=%s)", ErrExpectedAtLeastOneWebhook,
			kymaName, webhookConfig.Name)
	}
	idx := lookupWebhookConfigForCR(webhookConfig.Webhooks, watcher)
	if idx == -1 {
		return fmt.Errorf("%w: (kyma=%s, webconfig=%s, watcher=%s)", ErrWebhookConfigForWatcherNotFound,
			kymaName, webhookConfig.Name, watcher.Name)
	}
	return verifyWebhookConfig(webhookConfig.Webhooks[idx], watcher)
}

func lookupWebhookConfigForCR(webhooks []admissionregistrationv1.ValidatingWebhook, watcher *v1beta2.Watcher) int {
	cfgIdx := -1
	for idx, webhook := range webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) < 3 {
			continue
		}

		if watcher.Namespace == webhookNameParts[0] && watcher.Name == webhookNameParts[1] {
			return idx
		}
	}
	return cfgIdx
}

func verifyWebhookConfig(
	webhook admissionregistrationv1.ValidatingWebhook,
	watcherCR *v1beta2.Watcher,
) error {
	webhookNameParts := strings.Split(webhook.Name, ".")
	if len(webhookNameParts) != expectedWebhookNamePartsLength {
		return fmt.Errorf("%w: (webhook=%s)", ErrWebhookNamePartsNumberMismatch, webhook.Name)
	}
	watcherNamespace := webhookNameParts[0]
	watcherName := webhookNameParts[1]
	if watcherNamespace != watcherCR.Namespace || watcherName != watcherCR.Name {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrWebhookCfgNameMismatch,
			client.ObjectKeyFromObject(watcherCR), client.ObjectKey{
				Namespace: watcherNamespace,
				Name:      watcherName,
			})
	}
	expectedModuleName, exists := watcherCR.Labels[v1beta2.ManagedBy]
	if !exists {
		return fmt.Errorf("%w: (labels=%v)", ErrManagedByLabelNotFound, watcherCR.Labels)
	}
	expectedSvcPath := fmt.Sprintf(servicePathTpl, expectedModuleName)
	if *webhook.ClientConfig.Service.Path != expectedSvcPath {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrSvcPathMismatch,
			expectedSvcPath, *webhook.ClientConfig.Service.Path)
	}
	if !reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watcherCR.Spec.LabelsToWatch) {
		return fmt.Errorf("%w: (expected=%v, got=%v)", ErrWatchLabelsMismatch,
			watcherCR.Spec.LabelsToWatch, webhook.ObjectSelector.MatchLabels)
	}
	expectedResources := watcher.ResolveWebhookRuleResources(watcherCR.Spec.ResourceToWatch.Resource,
		watcherCR.Spec.Field)
	if webhook.Rules[0].Resources[0] != expectedResources[0] {
		return fmt.Errorf("%w: (expected=%s, got=%s)", ErrResourcesMismatch,
			expectedResources[0], webhook.Rules[0].Resources[0])
	}
	return nil
}

func isWatcherCrDeletionFinished(watcherCR client.Object) bool {
	err := controlPlaneClient.Get(suiteCtx, client.ObjectKeyFromObject(watcherCR), watcherCR)
	return util.IsNotFound(err)
}
