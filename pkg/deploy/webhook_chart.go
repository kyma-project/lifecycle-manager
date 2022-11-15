package deploy

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	moduleLib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	moduleLibTypes "github.com/kyma-project/module-manager/operator/pkg/types"
)

type Mode string

const (
	ModeInstall            = Mode("install")
	ModeUninstall          = Mode("uninstall")
	customConfigKey        = "modules"
	ReleaseName            = "skr"
	IstioSytemNs           = "istio-system"
	IngressServiceName     = "istio-ingressgateway"
	DeploymentNameTpl      = "%s-webhook"
	triggerLabelTimeFormat = "200601021504050700"
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

func ResolveSKRChartResourceName(resourceNameTpl string) string {
	return fmt.Sprintf(resourceNameTpl, ReleaseName)
}

func prepareInstallInfo(ctx context.Context, chartPath, releaseName string, restConfig *rest.Config,
	restClient client.Client, argsVals map[string]interface{},
) moduleLib.InstallInfo {
	return moduleLib.InstallInfo{
		Ctx: ctx,
		ChartInfo: &moduleLib.ChartInfo{
			ChartPath:   chartPath,
			ReleaseName: releaseName,
			Flags: moduleLibTypes.ChartFlags{
				SetFlags: argsVals,
			},
		},
		ClusterInfo: moduleLibTypes.ClusterInfo{
			Client: restClient,
			Config: restConfig,
		},
	}
}
