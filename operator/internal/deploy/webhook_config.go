package deploy

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	customConfigKey                     = "modules"
	kubeconfigKey                       = "config"
	servicePathTpl                      = "/validate/%s"
	webhookNameTpl                      = "%s.operator.kyma-project.io"
	webhookConfigNameTpl                = "%s-webhook"
	specSubresources                    = "*"
	statusSubresources                  = "*/status"
	configuredWebhooksDeletionThreshold = 1
	expectedWebhookNamePartsLength      = 4
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func UpdateWebhookConfig(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, inClusterCfg *rest.Config, k8sClient client.Client,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, k8sClient, inClusterCfg)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = updateWebhookConfigOrInstallSKRChart(ctx, chartPath, releaseName, obj, restCfg, k8sClient)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil for at least one SKR, reconciliation will be retriggered after requeue interval
	return err
}

func RemoveWebhookConfig(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, inClusterCfg *rest.Config, k8sClient client.Client,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, k8sClient, inClusterCfg)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = removeWebhookConfig(ctx, chartPath, releaseName, obj, restCfg, k8sClient)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil for at least one SKR, reconciliation will be retriggered after requeue interval
	return err
}

func IsWebhookConfigured(ctx context.Context, obj *watcherv1alpha1.Watcher, restConfig *rest.Config,
	releaseName string,
) bool {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return false
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      webhookConfigName(releaseName),
	}, webhookConfig)
	if err != nil {
		return false
	}
	if len(webhookConfig.Webhooks) < 1 {
		return false
	}
	idx := lookupWebhookConfigForCR(webhookConfig.Webhooks, obj)
	if idx != -1 {
		// TODO: replace with deepequal?
		return verifyWebhookConfig(webhookConfig.Webhooks[idx], obj)
	}
	return false
}

func IsWebhookDeployed(ctx context.Context, restConfig *rest.Config, releaseName string) bool {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return false
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      webhookConfigName(releaseName),
	}, webhookConfig)
	return err == nil
}

func verifyWebhookConfig(
	webhook admissionv1.ValidatingWebhook,
	watcherCR *watcherv1alpha1.Watcher,
) bool {
	webhookNameParts := strings.Split(webhook.Name, ".")
	if len(webhookNameParts) != expectedWebhookNamePartsLength {
		return false
	}
	moduleName := webhookNameParts[0]
	expectedModuleName, exists := watcherCR.Labels[watcherv1alpha1.ManagedBylabel]
	if !exists {
		return false
	}
	if moduleName != expectedModuleName {
		return false
	}
	if *webhook.ClientConfig.Service.Path != fmt.Sprintf(servicePathTpl, moduleName) {
		return false
	}

	if !reflect.DeepEqual(webhook.ObjectSelector.MatchLabels, watcherCR.Spec.LabelsToWatch) {
		return false
	}
	if watcherCR.Spec.Field == watcherv1alpha1.StatusField && webhook.Rules[0].Resources[0] != statusSubresources {
		return false
	}
	if watcherCR.Spec.Field == watcherv1alpha1.SpecField && webhook.Rules[0].Resources[0] != specSubresources {
		return false
	}

	return true
}

func updateWebhookConfigOrInstallSKRChart(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, restConfig *rest.Config, k8sClient client.Client,
) error {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      webhookConfigName(releaseName),
	}, webhookConfig)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if kerrors.IsNotFound(err) {
		// install chart
		return InstallSKRWebhook(ctx, chartPath, releaseName, obj, restConfig)
	}
	// generate webhook config from CR and update webhook config resource
	if len(webhookConfig.Webhooks) < 1 {
		return fmt.Errorf("failed to get base webhook config")
	}
	idx := lookupWebhookConfigForCR(webhookConfig.Webhooks, obj)
	if idx != -1 {
		// update existing webhook
		webhookConfig.Webhooks[idx] = generateWebhookConfigForCR(webhookConfig.Webhooks[idx], obj)
		return remoteClient.Update(ctx, webhookConfig)
	}

	webhookConfig.Webhooks = append(webhookConfig.Webhooks, generateWebhookConfigForCR(webhookConfig.Webhooks[0], obj))
	return remoteClient.Update(ctx, webhookConfig)
}

func lookupWebhookConfigForCR(webhooks []admissionv1.ValidatingWebhook, obj *watcherv1alpha1.Watcher) int {
	cfgIdx := -1
	for idx, webhook := range webhooks {
		webhookNameParts := strings.Split(webhook.Name, ".")
		if len(webhookNameParts) == 0 {
			continue
		}
		moduleName := webhookNameParts[0]
		objModuleName := obj.GetModuleName()
		if moduleName == objModuleName {
			return idx
		}
	}
	return cfgIdx
}

func generateWebhookConfigForCR(baseCfg admissionv1.ValidatingWebhook, obj *watcherv1alpha1.Watcher,
) admissionv1.ValidatingWebhook {
	watcherCrWebhookCfg := baseCfg.DeepCopy()
	moduleName := obj.GetModuleName()
	watcherCrWebhookCfg.Name = fmt.Sprintf(webhookNameTpl, moduleName)
	if obj.Spec.LabelsToWatch != nil {
		watcherCrWebhookCfg.ObjectSelector.MatchLabels = obj.Spec.LabelsToWatch
	}
	servicePath := fmt.Sprintf(servicePathTpl, moduleName)
	watcherCrWebhookCfg.ClientConfig.Service.Path = &servicePath
	if obj.Spec.Field == watcherv1alpha1.StatusField {
		watcherCrWebhookCfg.Rules[0].Resources[0] = statusSubresources
		return *watcherCrWebhookCfg
	}
	watcherCrWebhookCfg.Rules[0].Resources[0] = specSubresources
	return *watcherCrWebhookCfg
}

func removeWebhookConfig(ctx context.Context, chartPath, releaseName string,
	obj *watcherv1alpha1.Watcher, restConfig *rest.Config, k8sClient client.Client,
) error {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      webhookConfigName(releaseName),
	}, webhookConfig)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if kerrors.IsNotFound(err) {
		return nil
	}
	numberOfWebhooks := len(webhookConfig.Webhooks)
	if numberOfWebhooks <= configuredWebhooksDeletionThreshold {
		// this watcher CR is the latest CR configured on the SKR webhook
		// remove the webhook configuration
		return remoteClient.Delete(ctx, webhookConfig)
	}
	cfgIdx := lookupWebhookConfigForCR(webhookConfig.Webhooks, obj)
	if cfgIdx != -1 {
		// remove corresponding config from webhook config resource
		copy(webhookConfig.Webhooks[cfgIdx:], webhookConfig.Webhooks[cfgIdx+1:])
		webhookConfig.Webhooks[numberOfWebhooks-1] = admissionv1.ValidatingWebhook{}
		webhookConfig.Webhooks = webhookConfig.Webhooks[:len(webhookConfig.Webhooks)-1]
		return remoteClient.Update(ctx, webhookConfig)
	}
	return nil
}

func getSKRRestConfigs(ctx context.Context, reader client.Reader, inClusterCfg *rest.Config,
) (map[string]*rest.Config, error) {
	kymaCRs := &kyma.KymaList{}
	err := reader.List(ctx, kymaCRs)
	if err != nil {
		return nil, err
	}
	if len(kymaCRs.Items) == 0 {
		return nil, fmt.Errorf("no kymas found")
	}
	restCfgMap := make(map[string]*rest.Config, len(kymaCRs.Items))
	for _, kymaCr := range kymaCRs.Items {
		if kymaCr.Spec.Sync.Strategy == kyma.SyncStrategyLocalClient || !kymaCr.Spec.Sync.Enabled {
			restCfgMap[kymaCr.Name] = inClusterCfg
			continue
		}
		secret := &v1.Secret{}
		//nolint:gosec
		err = reader.Get(ctx, client.ObjectKeyFromObject(&kymaCr), secret)
		if err != nil {
			return nil, err
		}
		restCfg, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[kubeconfigKey])
		if err == nil {
			restCfgMap[kymaCr.Name] = restCfg
		}
	}

	return restCfgMap, nil
}

func webhookConfigName(releaseName string) string {
	return fmt.Sprintf(webhookConfigNameTpl, releaseName)
}
