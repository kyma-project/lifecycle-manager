package deploy

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
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

func (m *SKRWebhookChartManager) IsSkrChartRemoved(ctx context.Context, kyma *v1alpha1.Kyma,
	remoteClientCache *remote.ClientCache, kcpClient client.Client,
) bool {
	skrClient, err := remote.NewRemoteClient(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, remoteClientCache)
	if err != nil {
		return false
	}
	err = skrClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      ResolveSKRChartResourceName(DeploymentNameTpl),
	}, &appsv1.Deployment{})
	return apierrors.IsNotFound(err)
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
