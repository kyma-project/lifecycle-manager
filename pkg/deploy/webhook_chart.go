package deploy

import (
	"context"
	"errors"
	"fmt"
	modulelabels "github.com/kyma-project/module-manager/operator/pkg/labels"
	moduletypes "github.com/kyma-project/module-manager/operator/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Mode string

const (
	ModeInstall             = Mode("install")
	ModeUninstall           = Mode("uninstall")
	customConfigKey         = "modules"
	ReleaseNameSuffix       = "skr"
	IstioSytemNs            = "istio-system"
	IngressServiceName      = "istio-ingressgateway"
	DeploymentNameTpl       = "%s-webhook"
	releaseNameTpl          = "%s-%s-skr"
	triggerLabelTimeFormat  = "200601021504050700"
	staticWatcherConfigName = "static-watcher-config-name"
)

var (
	ErrSKRWebhookHasNotBeenInstalled = errors.New("skr webhook resources have not been installed")
	ErrSKRWebhookWasNotRemoved       = errors.New("installed skr webhook resources were not removed")
	ErrLoadBalancerIPIsNotAssigned   = errors.New("load balancer service external ip is not assigned")
	ErrFoundZeroWatchers             = errors.New("found 0 watcher resources, expected at least 1")
)

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
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
			BaseResource: cachingKeyBaseResource(kymaObjKey),
		},
		ClusterInfo: moduletypes.ClusterInfo{
			Client: restClient,
			Config: restConfig,
		},
	}
}
func cachingKeyBaseResource(kymaObjKey client.ObjectKey) *unstructured.Unstructured {
	baseRes := &unstructured.Unstructured{}
	baseRes.SetLabels(map[string]string{
		modulelabels.CacheKey: kymaObjKey.Name,
	})
	baseRes.SetNamespace(kymaObjKey.Namespace)
	baseRes.SetName(staticWatcherConfigName)
	return baseRes
}
