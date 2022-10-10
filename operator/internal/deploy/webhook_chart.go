package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

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
	IstioSytemNs                        = "istio-system"
	IngressServiceName                  = "istio-ingressgateway"
)

var (
	ErrSKRWebhookAreNotReady         = errors.New("installed skr webhook resources are not ready")
	ErrSKRWebhookHasNotBeenInstalled = errors.New("installed skr webhook resources have not been installed")
	ErrLoadBalancerIPIsNotAssigned   = errors.New("load balancer service external ip is not assigned")
)

func InstallSKRWebhook(ctx context.Context, chartPath, releaseName string,
	obj *v1alpha1.Watcher, restConfig *rest.Config, kcpClient client.Client,
) error {
	restClient, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}
	argsVals, err := generateHelmChartArgsForCR(ctx, obj, kcpClient)
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

func generateHelmChartArgsForCR(ctx context.Context, obj *v1alpha1.Watcher, kcpClient client.Client,
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
		"kcpAddr":       resolvedKcpAddr,
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
	args := make(map[string]map[string]interface{}, 0)
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
		return ErrSKRWebhookHasNotBeenInstalled
	}
	ready, err := ops.VerifyResources(deployInfo)
	if err != nil {
		return fmt.Errorf("failed to verify webhook resources: %w", err)
	}
	if !ready {
		return ErrSKRWebhookAreNotReady
	}
	return nil
}

func resolveKcpAddr(ctx context.Context, kcpClient client.Client) (string, error) {
	// as fallback get external IP from the ISTIO load balancer external IP
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
