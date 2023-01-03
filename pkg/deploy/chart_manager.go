package deploy

import (
	"context"
	"fmt"
	"k8s.io/client-go/rest"
	"net"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"

	moduletypes "github.com/kyma-project/module-manager/pkg/types"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	modulelib "github.com/kyma-project/module-manager/pkg/manifest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

type SKRWebhookChartManager interface {
	Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error)
	Remove(ctx context.Context, kyma *v1alpha1.Kyma) error
}

type SKRWebhookChartManagerImpl struct {
	cache   moduletypes.RendererCache
	config  *SkrChartManagerConfig
	kcpAddr string
}

type SkrChartManagerConfig struct {
	// WebhookChartPath represents the path of the webhook chart
	// to be installed on SKR clusters upon reconciling kyma CRs.
	WebhookChartPath        string
	SkrWebhookMemoryLimits  string
	SkrWebhookCPULimits     string
	IstioNamespace          string
	IstioIngressServiceName string
}

func NewSKRWebhookChartManagerImpl(kcpRestConfig *rest.Config, config *SkrChartManagerConfig) (*SKRWebhookChartManagerImpl, error) {
	resolvedKcpAddr, err := resolveKcpAddr(kcpRestConfig, config)
	if err != nil {
		return nil, err
	}
	return &SKRWebhookChartManagerImpl{
		cache:   modulelib.NewRendererCache(),
		config:  config,
		kcpAddr: resolvedKcpAddr,
	}, nil
}

func (m *SKRWebhookChartManagerImpl) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	chartArgsValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, m.config, m.kcpAddr)
	if err != nil {
		return true, err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath,
		syncContext.RuntimeRestConfig, syncContext.RuntimeClient, chartArgsValues, kymaObjKey)
	logger.V(1).Info("following watcher configs will be installed",
		"configs", prettyPrintSetFlags(skrWatcherInstallInfo.ChartInfo.Flags.SetFlags[customConfigKey]))
	installed, err := modulelib.InstallChart(
		modulelib.OperationOptions{
			Logger:             logger,
			InstallInfo:        skrWatcherInstallInfo,
			ResourceTransforms: nil,
			PostRuns:           nil,
			Cache:              m.cache,
		})
	if err != nil {
		kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionFalse)
		return true, fmt.Errorf("failed to install webhook chart: %w", err)
	}
	if !installed {
		kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionFalse)
		return true, ErrSKRWebhookNotInstalled
	}
	// if err := CheckWebhookCABundleConsistency(ctx, syncContext.RuntimeClient, kymaObjKey); err != nil {
	// 	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionFalse)
	// 	return true, fmt.Errorf("failed to install webhook chart: %w", err)
	// }
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook chart",
		"release-name", skrWatcherInstallInfo.ChartInfo.ReleaseName)
	return false, nil
}

func (m *SKRWebhookChartManagerImpl) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	argsValues, err := generateHelmChartArgs(ctx, syncContext.ControlPlaneClient, m.config, m.kcpAddr)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath,
		syncContext.RuntimeRestConfig, syncContext.RuntimeClient, argsValues, kymaObjKey)
	uninstalled, err := modulelib.UninstallChart(
		modulelib.OperationOptions{
			Logger:             logger,
			InstallInfo:        skrWatcherInstallInfo,
			ResourceTransforms: nil,
			PostRuns:           nil,
			Cache:              nil,
		})
	if err != nil {
		return fmt.Errorf("failed to uninstall webhook config: %w", err)
	}
	if !uninstalled {
		return ErrSKRWebhookWasNotRemoved
	}
	logger.Info("successfully uninstalled webhook chart",
		"release-name", skrWatcherInstallInfo.ChartInfo.ReleaseName)
	return nil
}

func generateHelmChartArgs(ctx context.Context, kcpClient client.Client,
	managerConfig *SkrChartManagerConfig, kcpAddr string,
) (map[string]interface{}, error) {
	customConfigValue := ""
	watcherList := &v1alpha1.WatcherList{}
	if err := kcpClient.List(ctx, watcherList); err != nil {
		return nil, fmt.Errorf("error listing watcher CRs: %w", err)
	}
	watchers := watcherList.Items
	if len(watchers) != 0 {
		chartCfg := generateWatchableConfigs(watchers)
		chartConfigBytes, err := k8syaml.Marshal(chartCfg)
		if err != nil {
			return nil, err
		}
		customConfigValue = string(chartConfigBytes)
	}

	return map[string]interface{}{
		"kcpAddr":               kcpAddr,
		"resourcesLimitsMemory": managerConfig.SkrWebhookMemoryLimits,
		"resourcesLimitsCPU":    managerConfig.SkrWebhookCPULimits,
		customConfigKey:         customConfigValue,
	}, nil
}

func resolveKcpAddr(kcpConfig *rest.Config,
	managerConfig *SkrChartManagerConfig,
) (string, error) {
	ctx := context.TODO()
	// Get public KCP IP from the ISTIO load balancer external IP
	kcpClient, err := client.New(kcpConfig, client.Options{})
	if err != nil {
		return "", err
	}
	loadBalancerService := &corev1.Service{}
	if err := kcpClient.Get(ctx, client.ObjectKey{
		Name:      managerConfig.IstioIngressServiceName,
		Namespace: managerConfig.IstioNamespace,
	}, loadBalancerService); err != nil {
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
