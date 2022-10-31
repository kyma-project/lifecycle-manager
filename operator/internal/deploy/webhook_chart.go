package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	moduleLib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	moduleLibTypes "github.com/kyma-project/module-manager/operator/pkg/types"
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
	IstioSytemNs                        = "istio-system"
	IngressServiceName                  = "istio-ingressgateway"
)

var (
	ErrSKRWebhookNotReady            = errors.New("installed skr webhook resources are not ready")
	ErrSKRWebhookHasNotBeenInstalled = errors.New("skr webhook resources have not been installed")
	ErrSKRWebhookWasNotRemoved       = errors.New("installed skr webhook resources were not removed")
	ErrLoadBalancerIPIsNotAssigned   = errors.New("load balancer service external ip is not assigned")
)

func installSKRWebhook(ctx context.Context, chartPath, releaseName string, obj *v1alpha1.Watcher,
	restConfig *rest.Config, kcpClient client.Client, skrWebhookMemoryLimits, skrWebhookCPULimits string,
) error {
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	argsVals, err := generateHelmChartArgsForCR(ctx, obj, kcpClient, skrWebhookMemoryLimits, skrWebhookCPULimits)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, chartPath, releaseName, restConfig, restClient, argsVals)
	return installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeInstall)
}

func removeSKRWebhook(ctx context.Context, chartPath, releaseName string,
	obj *v1alpha1.Watcher, restConfig *rest.Config, kcpClient client.Client,
	skrWebhookMemoryLimits, skrWebhookCPULimits string,
) error {
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	argsVals, err := generateHelmChartArgsForCR(ctx, obj, kcpClient, skrWebhookMemoryLimits, skrWebhookCPULimits)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, chartPath, releaseName, restConfig, restClient, argsVals)
	return installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeUninstall)
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

func generateHelmChartArgsForCR(ctx context.Context, obj *v1alpha1.Watcher, kcpClient client.Client,
	skrWebhookMemoryLimits string, skrWebhookCPULimits string,
) (map[string]interface{}, error) {
	resolvedKcpAddr, err := resolveKcpAddr(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	chartCfg := generateWatchableConfigForCR(obj)
	bytes, err := k8syaml.Marshal(chartCfg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"kcpAddr":               resolvedKcpAddr,
		"resourcesLimitsMemory": skrWebhookMemoryLimits,
		"resourcesLimitsCPU":    skrWebhookCPULimits,
		customConfigKey:         string(bytes),
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

func installOrRemoveChartOnSKR(ctx context.Context, deployInfo moduleLib.InstallInfo, mode Mode,
) error {
	logger := logf.FromContext(ctx)
	if mode == ModeUninstall {
		uninstalled, err := moduleLib.UninstallChart(&logger, deployInfo, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to uninstall webhook config: %w", err)
		}
		if !uninstalled {
			return ErrSKRWebhookWasNotRemoved
		}
		return nil
	}
	installed, err := moduleLib.InstallChart(&logger, deployInfo, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to install webhook config: %w", err)
	}
	if !installed {
		return ErrSKRWebhookHasNotBeenInstalled
	}
	ready, err := moduleLib.ConsistencyCheck(&logger, deployInfo, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to verify webhook resources: %w", err)
	}
	if !ready {
		return ErrSKRWebhookNotReady
	}
	return nil
}

func resolveKcpAddr(ctx context.Context, kcpClient client.Client) (string, error) {
	// Get external IP from the ISTIO load balancer external IP
	loadBalancerService := &v1.Service{}
	if err := kcpClient.Get(ctx, client.ObjectKey{Name: IngressServiceName, Namespace: IstioSytemNs},
		loadBalancerService); err != nil {
		return "", err
	}
	if len(loadBalancerService.Status.LoadBalancer.Ingress) == 0 {
		return "", ErrLoadBalancerIPIsNotAssigned
	}
	externalIP := loadBalancerService.Status.LoadBalancer.Ingress[0].IP
	var port int32
	for _, loadBalancerPort := range loadBalancerService.Spec.Ports {
		if loadBalancerPort.Name == "http2" {
			port = loadBalancerPort.Port
			break
		}
	}
	return net.JoinHostPort(externalIP, strconv.Itoa(int(port))), nil
}
