package deploy

import (
	"context"
	"fmt"
	"net"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"helm.sh/helm/v3/pkg/cli"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	modulelib "github.com/kyma-project/module-manager/operator/pkg/manifest"

	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/module-manager/operator/pkg/custom"

	k8syaml "sigs.k8s.io/yaml"
)

type Mode string

const (
	ModeInstall                         = Mode("install")
	ModeUninstall                       = Mode("uninstall")
	customConfigKey                     = "modules"
	kubeconfigKey                       = "config"
	servicePathTpl                      = "/validate/%s"
	webhookNameTpl                      = "%s.operator.kyma-project.io"
	ReleaseName                         = "skr"
	specSubresources                    = "*"
	statusSubresources                  = "*/status"
	configuredWebhooksDeletionThreshold = 1
	expectedWebhookNamePartsLength      = 4
	istioSytemNs                        = "istio-system"
	ingressServiceName                  = "istio-ingressgateway"
)

func InstallSKRWebhook(ctx context.Context, chartPath, releaseName string,
	obj *v1alpha1.Watcher, restConfig *rest.Config, kcpAddr string,
) error {
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	argsVals, err := generateHelmChartArgsForCR(ctx, obj, kcpAddr, restClient)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(chartPath, releaseName, restConfig, restClient)
	return installOrRemoveChartOnSKR(ctx, restConfig, releaseName, argsVals, skrWatcherInstallInfo, ModeInstall)
}

func prepareInstallInfo(chartPath, releaseName string, restConfig *rest.Config, restClient client.Client,
) modulelib.InstallInfo {
	return modulelib.InstallInfo{
		ChartInfo: &modulelib.ChartInfo{
			ChartPath:   chartPath,
			ReleaseName: releaseName,
		},
		ClusterInfo: custom.ClusterInfo{
			Client: restClient,
			Config: restConfig,
		},
		CheckFn: func(ctx context.Context, u *unstructured.Unstructured, logger *logr.Logger, info custom.ClusterInfo,
		) (bool, error) {
			return true, nil
		},
	}
}

func generateHelmChartArgsForCR(ctx context.Context, obj *v1alpha1.Watcher, kcpAddr string,
	restClient client.Client) (map[string]interface{}, error) {
	resolvedKcpAddr, err := resolveKcpAddr(ctx, kcpAddr, restClient)
	if err != nil {
		return nil, err
	}
	chartCfg := generateWatchableConfigForCR(obj)
	bytes, err := k8syaml.Marshal(chartCfg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"kcp.addr":      resolvedKcpAddr,
		customConfigKey: string(bytes),
	}, nil
}

func generateWatchableConfigForCR(obj *v1alpha1.Watcher) map[string]WatchableConfig {
	statusOnly := obj.Spec.Field == v1alpha1.StatusField
	return map[string]WatchableConfig{
		obj.GetModuleName(): {
			Labels:     obj.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		},
	}
}

func installOrRemoveChartOnSKR(ctx context.Context, restConfig *rest.Config, releaseName string,
	argsVals map[string]interface{}, deployInfo modulelib.InstallInfo, mode Mode,
) error {
	logger := logf.FromContext(ctx)
	args := make(map[string]map[string]interface{}, 1)
	args["set"] = argsVals
	ops, err := modulelib.NewOperations(&logger, restConfig, releaseName,
		&cli.EnvSettings{}, args, nil)
	if err != nil {
		return err
	}
	if mode == ModeUninstall {
		uninstalled, err := ops.Uninstall(deployInfo)
		if err != nil {
			return fmt.Errorf("failed to uninstall webhook config: %w", err)
		}
		if !uninstalled {
			//nolint:goerr113
			return fmt.Errorf("waiting for skr webhook resources to be deleted")
		}
		return nil
	}
	installed, err := ops.Install(deployInfo)
	if err != nil {
		return fmt.Errorf("failed to install webhook config: %w", err)
	}
	if !installed {
		//nolint:goerr113
		return fmt.Errorf("installed skr webhook resources are not ready")
	}
	return nil
}

func resolveKcpAddr(ctx context.Context, kcpAddr string, restClient client.Client) (string, error) {
	if kcpAddr == "" {
		// as fallback get external IP from the ISTIO load balancer external IP
		loadBalancerService := &v1.Service{}
		if err := restClient.Get(ctx, client.ObjectKey{Name: ingressServiceName, Namespace: istioSytemNs},
			loadBalancerService); err != nil {
			return "", err
		}
		ip := loadBalancerService.Status.LoadBalancer.Ingress[0].IP
		var port int32
		for _, loadBalancerPort := range loadBalancerService.Spec.Ports {
			if loadBalancerPort.Name == "http2" {
				port = loadBalancerPort.Port
				break
			}
		}
		kcpAddr = net.JoinHostPort(ip, string(port))
	}
	return kcpAddr, nil
}
