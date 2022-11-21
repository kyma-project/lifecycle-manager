package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	moduletypes "github.com/kyma-project/module-manager/operator/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	modulelib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

const caCertificateSecretKey = "ca.crt"

var ErrExpectedExactlyOneSKRConfig = errors.New("expected exactly one SKR config")

type SKRWebhookChartManager interface {
	InstallWebhookChart(ctx context.Context, syncContext *remote.KymaSynchronizationContext) (bool, error)
	RemoveWebhookChart(ctx context.Context, syncContext *remote.KymaSynchronizationContext) error
}

type DisabledSKRWebhookChartManager struct{}

// ResolveSKRWebhookChartManager resolves to enabled or disabled chart manager.
// nolint: ireturn
func ResolveSKRWebhookChartManager(
	isWatcherEnabled bool,
	skrConfigs ...*SkrChartConfig,
) (SKRWebhookChartManager, error) {
	if isWatcherEnabled && len(skrConfigs) != 1 {
		return nil, ErrExpectedExactlyOneSKRConfig
	}
	if !isWatcherEnabled {
		return &DisabledSKRWebhookChartManager{}, nil
	}

	return NewEnabledSKRWebhookChartManager(skrConfigs[0]), nil
}

func (m *DisabledSKRWebhookChartManager) InstallWebhookChart(_ context.Context,
	_ *remote.KymaSynchronizationContext,
) (bool, error) {
	return false, nil
}

func (m *DisabledSKRWebhookChartManager) RemoveWebhookChart(_ context.Context,
	_ *remote.KymaSynchronizationContext,
) error {
	return nil
}

type EnabledSKRWebhookChartManager struct {
	cache   moduletypes.RendererCache
	config  *SkrChartConfig
	kcpAddr string
}

type SkrChartConfig struct {
	// WebhookChartPath represents the path of the webhook chart
	// to be installed on SKR clusters upon reconciling kyma CRs.
	WebhookChartPath       string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
}

func NewEnabledSKRWebhookChartManager(config *SkrChartConfig) *EnabledSKRWebhookChartManager {
	return &EnabledSKRWebhookChartManager{
		cache:  modulelib.NewRendererCache(),
		config: config,
	}
}

func (m *EnabledSKRWebhookChartManager) InstallWebhookChart(ctx context.Context,
	syncContext *remote.KymaSynchronizationContext,
) (bool, error) {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(syncContext.ControlPlaneKyma)
	skrCfg, err := remote.GetRemoteRestConfig(ctx, syncContext.ControlPlaneClient, kymaObjKey,
		syncContext.ControlPlaneKyma.Spec.Sync.Strategy)
	if err != nil {
		return true, err
	}
	chartArgsValues, err := m.generateHelmChartArgs(ctx, syncContext.ControlPlaneClient)
	if err != nil {
		return true, err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath, skrCfg, syncContext.RuntimeClient,
		chartArgsValues, kymaObjKey)
	installed, err := modulelib.InstallChart(&logger, skrWatcherInstallInfo, nil, m.cache)
	if err != nil {
		return true, fmt.Errorf("failed to install webhook config: %w", err)
	}
	if !installed {
		return true, ErrSKRWebhookNotInstalled
	}
	syncContext.ControlPlaneKyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	logger.Info("successfully installed webhook chart",
		"release-name", skrWatcherInstallInfo.ChartInfo.ReleaseName)
	logger.V(1).Info("following modules were installed",
		"modules", skrWatcherInstallInfo.ChartInfo.Flags.SetFlags[customConfigKey])
	return false, nil
}

func (m *EnabledSKRWebhookChartManager) RemoveWebhookChart(ctx context.Context,
	syncContext *remote.KymaSynchronizationContext,
) error {
	logger := logf.FromContext(ctx)
	kymaObjKey := client.ObjectKeyFromObject(syncContext.ControlPlaneKyma)
	skrCfg, err := remote.GetRemoteRestConfig(ctx, syncContext.ControlPlaneClient, kymaObjKey,
		syncContext.ControlPlaneKyma.Spec.Sync.Strategy)
	if err != nil {
		return err
	}
	argsVals, err := m.generateHelmChartArgs(ctx, syncContext.ControlPlaneClient)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath, skrCfg,
		syncContext.RuntimeClient, argsVals, kymaObjKey)
	uninstalled, err := modulelib.UninstallChart(&logger, skrWatcherInstallInfo, nil, m.cache)
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

func (m *EnabledSKRWebhookChartManager) generateHelmChartArgs(ctx context.Context,
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

func (m *EnabledSKRWebhookChartManager) resolveKcpAddr(ctx context.Context, kcpClient client.Client) (string, error) {
	if m.kcpAddr != "" {
		return m.kcpAddr, nil
	}
	// Get external IP from the ISTIO load balancer external IP
	loadBalancerService := &corev1.Service{}
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
	m.kcpAddr = net.JoinHostPort(externalIP, strconv.Itoa(int(port)))
	return m.kcpAddr, nil
}
