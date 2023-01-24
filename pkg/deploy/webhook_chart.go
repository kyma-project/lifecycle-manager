package deploy

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	moduleTypes "github.com/kyma-project/module-manager/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Mode string

const (
	ModeInstall                = Mode("install")
	ModeUninstall              = Mode("uninstall")
	customConfigKey            = "modules"
	ReleaseName                = "skr"
	IstioSytemNs               = "istio-system"
	IngressServiceName         = "istio-ingressgateway"
	DeploymentNameTpl          = "%s-webhook"
	defaultK3dLocalhostMapping = "host.k3d.internal"
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

func prepareInstallInfo(ctx context.Context, chartPath, releaseName string,
	clnt remote.Client, argsVals map[string]interface{},
) *moduleTypes.InstallInfo {
	return &moduleTypes.InstallInfo{
		Ctx: ctx,
		ResourceInfo: &moduleTypes.ResourceInfo{
			// TODO: replace by a meaningful resource
			BaseResource: &unstructured.Unstructured{},
		},
		ChartInfo: &moduleTypes.ChartInfo{
			ChartPath:   chartPath,
			ReleaseName: releaseName,
			Flags: moduleTypes.ChartFlags{
				SetFlags: argsVals,
			},
		},
		ClusterInfo: &moduleTypes.ClusterInfo{
			Client: clnt,
			Config: clnt.Config(),
		},
	}
}
