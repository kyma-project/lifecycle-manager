package deploy

import (
	"context"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"net"
	"strconv"

	moduletypes "github.com/kyma-project/module-manager/operator/pkg/types"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	modulelib "github.com/kyma-project/module-manager/operator/pkg/manifest"

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

func NewSKRWebhookChartManagerImpl(config *SkrChartManagerConfig) *SKRWebhookChartManagerImpl {
	return &SKRWebhookChartManagerImpl{
		cache:  modulelib.NewRendererCache(),
		config: config,
	}
}

func (m *SKRWebhookChartManagerImpl) Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	chartArgsValues, err := m.generateHelmChartArgs(ctx, syncContext.ControlPlaneClient)
	if err != nil {
		return true, err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath,
		syncContext.RuntimeRestConfig, syncContext.RuntimeClient, chartArgsValues, kymaObjKey)
	installed, err := modulelib.InstallChart(logger, skrWatcherInstallInfo, nil, m.cache)
	if err != nil {
		return true, fmt.Errorf("failed to install webhook config: %w", err)
	}
	if !installed {
		return true, ErrSKRWebhookNotInstalled
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook chart",
		"release-name", skrWatcherInstallInfo.ChartInfo.ReleaseName)
	logger.V(1).Info("following modules were installed",
		"modules", skrWatcherInstallInfo.ChartInfo.Flags.SetFlags[customConfigKey])
	return false, nil
}

func (m *SKRWebhookChartManagerImpl) Remove(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(kyma)
	syncContext := remote.SyncContextFromContext(ctx)
	argsValues, err := m.generateHelmChartArgs(ctx, syncContext.ControlPlaneClient)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath,
		syncContext.RuntimeRestConfig, syncContext.RuntimeClient, argsValues, kymaObjKey)
	uninstalled, err := modulelib.UninstallChart(logger, skrWatcherInstallInfo, nil, m.cache)
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

func (m *SKRWebhookChartManagerImpl) generateHelmChartArgs(ctx context.Context,
	kcpClient client.Client,
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

	kcpAddr, err := m.resolveKcpAddr(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"kcpAddr":               kcpAddr,
		"resourcesLimitsMemory": m.config.SkrWebhookMemoryLimits,
		"resourcesLimitsCPU":    m.config.SkrWebhookCPULimits,
		customConfigKey:         customConfigValue,
	}, nil
}

func (m *SKRWebhookChartManagerImpl) resolveKcpAddr(ctx context.Context, kcpClient client.Client) (string, error) {
	if m.kcpAddr != "" {
		return m.kcpAddr, nil
	}
	// Get external IP from the ISTIO load balancer external IP
	loadBalancerService := &corev1.Service{}
	if err := kcpClient.Get(ctx, client.ObjectKey{
		Name:      m.config.IstioIngressServiceName,
		Namespace: m.config.IstioNamespace,
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
	m.kcpAddr = net.JoinHostPort(externalIP, strconv.Itoa(int(port)))
	return m.kcpAddr, nil
}
