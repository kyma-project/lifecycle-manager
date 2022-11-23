package deploy

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	modulelabels "github.com/kyma-project/module-manager/operator/pkg/labels"
	moduletypes "github.com/kyma-project/module-manager/operator/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Mode string

const (
	customConfigKey                = "modules"
	WebhookCfgAndDeploymentNameTpl = "%s-webhook"
	releaseNameTpl                 = "%s-%s-skr"
	staticWatcherConfigName        = "static-watcher-config-name"
)

var (
	ErrSKRWebhookNotInstalled         = errors.New("skr webhook resources are not installed")
	ErrSKRWebhookWasNotRemoved        = errors.New("installed skr webhook resources were not removed")
	ErrLoadBalancerIPIsNotAssigned    = errors.New("load balancer service external ip is not assigned")
	ErrCABundleAndCaCertMismatchFound = errors.New("found CABundle and CA certificate mismatch")
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func generateWatchableConfigs(watchers []v1alpha1.Watcher) map[string]WatchableConfig {
	chartCfg := make(map[string]WatchableConfig, 0)
	for _, watcher := range watchers {
		statusOnly := watcher.Spec.Field == v1alpha1.StatusField
		chartCfg[watcher.GetModuleName()] = WatchableConfig{
			Labels:     watcher.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		}
	}
	return chartCfg
}

func ResolveSKRChartResourceName(resourceNameTpl string, kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(resourceNameTpl, skrChartReleaseName(kymaObjKey))
}

func skrChartReleaseName(kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(releaseNameTpl, kymaObjKey.Namespace, kymaObjKey.Name)
}

func prepareInstallInfo(ctx context.Context, chartPath string, restConfig *rest.Config,
	restClient client.Client, argsVals map[string]interface{}, kymaObjKey client.ObjectKey,
) moduletypes.InstallInfo {
	return moduletypes.InstallInfo{
		Ctx: ctx,
		ChartInfo: &moduletypes.ChartInfo{
			ChartPath:   chartPath,
			ReleaseName: skrChartReleaseName(kymaObjKey),
			Flags: moduletypes.ChartFlags{
				SetFlags: argsVals,
			},
		},
		ResourceInfo: moduletypes.ResourceInfo{
			BaseResource: watcherCachingBaseResource(kymaObjKey),
		},
		ClusterInfo: moduletypes.ClusterInfo{
			Client: restClient,
			Config: restConfig,
		},
	}
}

func watcherCachingBaseResource(kymaObjKey client.ObjectKey) *unstructured.Unstructured {
	baseRes := &unstructured.Unstructured{}
	baseRes.SetLabels(map[string]string{
		modulelabels.CacheKey: kymaObjKey.String(),
	})
	baseRes.SetNamespace(metav1.NamespaceDefault)
	baseRes.SetName(staticWatcherConfigName)
	return baseRes
}
