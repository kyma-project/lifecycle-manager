package deploy

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

const (
	webhookConfigNameTpl        = "%s-webhook"
	serviceAccountNameTpl       = "%s-webhook-sa"
	secretNameTpl               = "%s-webhook-tls"
	serviceAndDeploymentNameTpl = "%s-webhook"
	clusterRoleName             = "kyma-reader"
	clusterRoleBindingName      = "read-kymas"
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func UpdateWebhookConfig(ctx context.Context, chartPath string, obj *v1alpha1.Watcher, inClusterCfg *rest.Config,
	k8sClient client.Client, skrWebhookMemoryLimits string, skrWebhookCPULimits string,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, k8sClient, inClusterCfg)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = updateWebhookConfigOrInstallSKRChart(ctx, chartPath, obj, restCfg, k8sClient,
			skrWebhookMemoryLimits, skrWebhookCPULimits)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil for at least one SKR, reconciliation will be retriggered after requeue interval
	return err
}

func RemoveWebhookConfig(ctx context.Context, chartPath string, obj *v1alpha1.Watcher, inClusterCfg *rest.Config,
	k8sClient client.Client, skrWebhookMemoryLimits string, skrWebhookCPULimits string,
) error {
	restCfgs, err := getSKRRestConfigs(ctx, k8sClient, inClusterCfg)
	if err != nil {
		return err
	}
	for _, restCfg := range restCfgs {
		err = removeWebhookConfigOrUninstallChart(ctx, chartPath, obj, restCfg, k8sClient,
			skrWebhookMemoryLimits, skrWebhookCPULimits)
		if err != nil {
			continue
		}
	}
	// return err so that if err!=nil for at least one SKR, reconciliation will be triggered after requeue interval
	return err
}

func IsWebhookConfigured(obj *v1alpha1.Watcher, webhookConfig *admissionv1.ValidatingWebhookConfiguration) bool {
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

func GetDeployedWebhook(ctx context.Context, restConfig *rest.Config,
) (*admissionv1.ValidatingWebhookConfiguration, error) {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(webhookConfigNameTpl),
	}, webhookConfig)
	if err != nil {
		return nil, err
	}
	return webhookConfig, nil
}

func IsChartRemoved(ctx context.Context, k8sClient client.Client) bool {
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(webhookConfigNameTpl),
	}, &admissionv1.ValidatingWebhookConfiguration{})
	if !apierrors.IsNotFound(err) {
		return false
	}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(secretNameTpl),
	}, &corev1.Secret{})
	if !apierrors.IsNotFound(err) {
		return false
	}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(serviceAndDeploymentNameTpl),
	}, &appsv1.Deployment{})
	if !apierrors.IsNotFound(err) {
		return false
	}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(serviceAndDeploymentNameTpl),
	}, &corev1.Service{})
	if !apierrors.IsNotFound(err) {
		return false
	}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(serviceAccountNameTpl),
	}, &corev1.ServiceAccount{})
	if !apierrors.IsNotFound(err) {
		return false
	}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterRoleName,
	}, &rbacv1.ClusterRole{})
	if !apierrors.IsNotFound(err) {
		return false
	}
	err = k8sClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      clusterRoleBindingName,
	}, &rbacv1.ClusterRoleBinding{})
	return apierrors.IsNotFound(err)
}

func verifyWebhookConfig(
	webhook admissionv1.ValidatingWebhook,
	watcherCR *v1alpha1.Watcher,
) bool {
	webhookNameParts := strings.Split(webhook.Name, ".")
	if len(webhookNameParts) != expectedWebhookNamePartsLength {
		return false
	}
	moduleName := webhookNameParts[0]
	expectedModuleName, exists := watcherCR.Labels[v1alpha1.ManagedBylabel]
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
	if watcherCR.Spec.Field == v1alpha1.StatusField && webhook.Rules[0].Resources[0] != statusSubresources {
		return false
	}
	if watcherCR.Spec.Field == v1alpha1.SpecField && webhook.Rules[0].Resources[0] != specSubresources {
		return false
	}

	return true
}

func updateWebhookConfigOrInstallSKRChart(ctx context.Context, chartPath string, obj *v1alpha1.Watcher,
	restConfig *rest.Config, kcpClient client.Client, skrWebhookMemoryLimits string, skrWebhookCPULimits string,
) error {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}

	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(webhookConfigNameTpl),
	}, webhookConfig)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if apierrors.IsNotFound(err) {
		// install chart
		return installSKRWebhook(ctx, chartPath, ReleaseName, obj, restConfig, kcpClient,
			skrWebhookMemoryLimits, skrWebhookCPULimits)
	}
	// generate webhook config from CR and update webhook config resource
	if len(webhookConfig.Webhooks) < 1 {
		//nolint:goerr113
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

func lookupWebhookConfigForCR(webhooks []admissionv1.ValidatingWebhook, obj *v1alpha1.Watcher) int {
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

func generateWebhookConfigForCR(baseCfg admissionv1.ValidatingWebhook, obj *v1alpha1.Watcher,
) admissionv1.ValidatingWebhook {
	watcherCrWebhookCfg := baseCfg.DeepCopy()
	moduleName := obj.GetModuleName()
	watcherCrWebhookCfg.Name = fmt.Sprintf(webhookNameTpl, moduleName)
	//fixme: if LabelsToWatch is nil: make sure that ObjectSelector is nil
	if obj.Spec.LabelsToWatch != nil {
		watcherCrWebhookCfg.ObjectSelector.MatchLabels = obj.Spec.LabelsToWatch
	}
	servicePath := fmt.Sprintf(servicePathTpl, moduleName)
	watcherCrWebhookCfg.ClientConfig.Service.Path = &servicePath
	if obj.Spec.Field == v1alpha1.StatusField {
		watcherCrWebhookCfg.Rules[0].Resources[0] = statusSubresources
		return *watcherCrWebhookCfg
	}
	watcherCrWebhookCfg.Rules[0].Resources[0] = specSubresources
	return *watcherCrWebhookCfg
}

func removeWebhookConfigOrUninstallChart(ctx context.Context, chartPath string,
	obj *v1alpha1.Watcher, restConfig *rest.Config, kcpClient client.Client,
	skrWebhookMemoryLimits, skrWebhookCPULimits string,
) error {
	remoteClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = remoteClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(webhookConfigNameTpl),
	}, webhookConfig)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if apierrors.IsNotFound(err) {
		return nil
	}
	numberOfWebhooks := len(webhookConfig.Webhooks)
	if numberOfWebhooks <= configuredWebhooksDeletionThreshold {
		// this watcher CR is the latest CR configured on the SKR webhook
		// uninstall the webhook chart
		return removeSKRWebhook(ctx, chartPath, ReleaseName, obj, restConfig, kcpClient,
			skrWebhookMemoryLimits, skrWebhookCPULimits)
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
	kymaCRs := &v1alpha1.KymaList{}
	err := reader.List(ctx, kymaCRs)
	if err != nil {
		return nil, err
	}
	if len(kymaCRs.Items) == 0 {
		//nolint:goerr113
		return nil, fmt.Errorf("no kymas found")
	}
	restCfgMap := make(map[string]*rest.Config, len(kymaCRs.Items))
	for _, kymaCr := range kymaCRs.Items {
		if kymaCr.Spec.Sync.Strategy == v1alpha1.SyncStrategyLocalClient || !kymaCr.Spec.Sync.Enabled {
			restCfgMap[kymaCr.Name] = inClusterCfg
			continue
		}
		secret := &corev1.Secret{}
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

func resolveSKRChartResourceName(resourceNameTpl string) string {
	return fmt.Sprintf(resourceNameTpl, ReleaseName)
}
