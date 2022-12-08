package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	modulelib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

var ErrExpectedExactlyOneSKRConfig = errors.New("expected exactly one SKR config")

type SKRWebhookChartManager interface {
	InstallWebhookChart(ctx context.Context, kyma *v1alpha1.Kyma,
		remoteClientCache *remote.ClientCache, kcpClient client.Client) (bool, error)
	RemoveWebhookChart(ctx context.Context, kyma *v1alpha1.Kyma) error
}

type DisabledSKRWebhookChartManager struct{}

// ResolveSKRWebhookChartManager resolves to enabled or disabled chart manager.

func ResolveSKRWebhookChartManager( //nolint:ireturn
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

func (m *DisabledSKRWebhookChartManager) InstallWebhookChart(_ context.Context, _ *v1alpha1.Kyma,
	_ *remote.ClientCache, _ client.Client,
) (bool, error) {
	return false, nil
}

func (m *DisabledSKRWebhookChartManager) RemoveWebhookChart(_ context.Context, _ *v1alpha1.Kyma) error {
	return nil
}

type EnabledSKRWebhookChartManager struct {
	cache   *SkrChartClientCache
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
		cache:  NewSKRChartClientCache(),
		config: config,
	}
}

func (m *EnabledSKRWebhookChartManager) InstallWebhookChart(ctx context.Context, kyma *v1alpha1.Kyma,
	remoteClientCache *remote.ClientCache, kcpClient client.Client,
) (bool, error) {
	skrClient, err := remote.NewRemoteClient(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, remoteClientCache)
	if err != nil {
		return true, err
	}
	skrCfg, err := remote.GetRemoteRestConfig(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy)
	if err != nil {
		return true, err
	}
	argsVals, err := m.generateHelmChartArgs(ctx, kcpClient)
	if err != nil {
		return true, err
	}
	// TODO(khlifi411): make sure that validating-webhook-config resource is in sync with the secret configuration
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath, ReleaseName, skrCfg, skrClient, argsVals)
	err = m.installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeInstall)
	if err != nil {
		return true, err
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	return false, nil
}

func (m *EnabledSKRWebhookChartManager) RemoveWebhookChart(ctx context.Context, kyma *v1alpha1.Kyma) error {
	syncContext := remote.SyncContextFromContext(ctx)

	skrCfg, err := remote.GetRemoteRestConfig(ctx, syncContext.ControlPlaneClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy)
	if err != nil {
		return err
	}
	argsVals, err := m.generateHelmChartArgs(ctx, syncContext.ControlPlaneClient)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(ctx, m.config.WebhookChartPath, ReleaseName, skrCfg,
		syncContext.RuntimeClient, argsVals)
	return m.installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeUninstall)
}

func (m *EnabledSKRWebhookChartManager) generateHelmChartArgs(ctx context.Context,
	kcpClient client.Client,
) (map[string]interface{}, error) {
	watcherList := &v1alpha1.WatcherList{}
	if err := kcpClient.List(ctx, watcherList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("error listing watcher resources: %w", err)
	}
	if len(watcherList.Items) == 0 {
		return nil, ErrFoundZeroWatchers
	}
	chartCfg := generateWatchableConfigs(watcherList)
	bytes, err := k8syaml.Marshal(chartCfg)
	if err != nil {
		return nil, err
	}

	kcpAddr, err := m.resolveKcpAddr(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"triggerLabel":          time.Now().Format(triggerLabelTimeFormat),
		"kcpAddr":               kcpAddr,
		"resourcesLimitsMemory": m.config.SkrWebhookMemoryLimits,
		"resourcesLimitsCPU":    m.config.SkrWebhookCPULimits,
		customConfigKey:         string(bytes),
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

func generateWatchableConfigs(watcherList *v1alpha1.WatcherList) map[string]WatchableConfig {
	chartCfg := make(map[string]WatchableConfig, len(watcherList.Items))
	for _, watcher := range watcherList.Items {
		statusOnly := watcher.Spec.Field == v1alpha1.StatusField
		chartCfg[watcher.GetModuleName()] = WatchableConfig{
			Labels:     watcher.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		}
	}
	return chartCfg
}

func (m *EnabledSKRWebhookChartManager) installOrRemoveChartOnSKR(ctx context.Context,
	deployInfo modulelib.InstallInfo, mode Mode,
) error {
	logger := logf.FromContext(ctx)
	if mode == ModeUninstall {
		uninstalled, err := modulelib.UninstallChart(&logger, deployInfo, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to uninstall webhook config: %w", err)
		}
		if !uninstalled {
			return ErrSKRWebhookWasNotRemoved
		}

		return nil
	}
	// TODO(khlifi411): verify webhook configuration with watchers' configuration before re-installing the chart
	ready, err := modulelib.ConsistencyCheck(&logger, deployInfo, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to verify webhook resources: %w", err)
	}
	if ready {
		logger.V(1).Info("chart resources already installed, nothing to do!")
		return nil
	}
	installed, err := modulelib.InstallChart(&logger, deployInfo, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to install webhook config: %w", err)
	}
	if !installed {
		return ErrSKRWebhookHasNotBeenInstalled
	}
	return nil
}
