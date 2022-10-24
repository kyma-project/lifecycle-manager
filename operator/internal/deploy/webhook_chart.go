package deploy

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	"github.com/kyma-project/module-manager/operator/pkg/custom"
	modulelib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	"github.com/kyma-project/module-manager/operator/pkg/types"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type Mode string

const (
	ModeInstall                    = Mode("install")
	ModeUninstall                  = Mode("uninstall")
	customConfigKey                = "modules"
	servicePathTpl                 = "/validate/%s"
	ReleaseName                    = "skr"
	specSubresources               = "*"
	statusSubresources             = "*/status"
	expectedWebhookNamePartsLength = 4
	IstioSytemNs                   = "istio-system"
	IngressServiceName             = "istio-ingressgateway"
	webhookConfigNameTpl           = "%s-webhook"
	deploymentNameTpl              = "%s-webhook"
)

var (
	ErrSKRWebhookHasNotBeenInstalled = errors.New("skr webhook resources have not been installed")
	ErrSKRWebhookWasNotRemoved       = errors.New("installed skr webhook resources were not removed")
	ErrLoadBalancerIPIsNotAssigned   = errors.New("load balancer service external ip is not assigned")
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
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

func GetDeployedWebhook(ctx context.Context, kyma *v1alpha1.Kyma,
	remoteClientCache *remote.ClientCache, kcpClient client.Client,
) (*admissionv1.ValidatingWebhookConfiguration, error) {
	skrClient, err := remote.NewRemoteClient(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, remoteClientCache)
	if err != nil {
		return nil, err
	}
	webhookConfig := &admissionv1.ValidatingWebhookConfiguration{}
	err = skrClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(webhookConfigNameTpl),
	}, webhookConfig)
	if err != nil {
		return nil, err
	}
	return webhookConfig, nil
}

func (m *SKRChartManager) IsSkrChartRemoved(ctx context.Context, kyma *v1alpha1.Kyma,
	remoteClientCache *remote.ClientCache, kcpClient client.Client,
) bool {
	skrClient, err := remote.NewRemoteClient(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, remoteClientCache)
	if err != nil {
		return false
	}
	err = skrClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      resolveSKRChartResourceName(deploymentNameTpl),
	}, &appsv1.Deployment{})
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

func resolveSKRChartResourceName(resourceNameTpl string) string {
	return fmt.Sprintf(resourceNameTpl, ReleaseName)
}

func prepareInstallInfo(chartPath, releaseName string, restConfig *rest.Config, restClient client.Client,
	argsVals map[string]interface{},
) modulelib.InstallInfo {
	return modulelib.InstallInfo{
		ChartInfo: &modulelib.ChartInfo{
			ChartPath:   chartPath,
			ReleaseName: releaseName,
			Flags: types.ChartFlags{
				SetFlags: argsVals,
			},
		},
		ClusterInfo: custom.ClusterInfo{
			Client: restClient,
			Config: restConfig,
		},
	}
}
