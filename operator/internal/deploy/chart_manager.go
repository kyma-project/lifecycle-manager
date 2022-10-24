package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	modulelib "github.com/kyma-project/module-manager/operator/pkg/manifest"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	k8syaml "sigs.k8s.io/yaml"
)

type SKRChartManager struct {
	WebhookChartPath       string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
	KcpAddr                string
}

func NewSKRChartManager(chartPath, memoryLimits, cpuLimits string) (*SKRChartManager, error) {
	mgr := &SKRChartManager{
		WebhookChartPath:       chartPath,
		SkrWebhookMemoryLimits: memoryLimits,
		SkrWebhookCPULimits:    cpuLimits,
	}

	return mgr, nil
}

func (m *SKRChartManager) InstallWebhookChart(ctx context.Context, kyma *v1alpha1.Kyma,
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
	// TODO: verify webhook configuration with watchers' configuration before re-installing the chart
	// TODO: make sure that validating-webhook-config resource is in sync with the secret configuration
	skrWatcherInstallInfo := prepareInstallInfo(m.WebhookChartPath, ReleaseName, skrCfg, skrClient, argsVals)

	if err := installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeInstall); err != nil {
		return true, err
	}
	kyma.UpdateCondition(v1alpha1.ConditionReasonSKRWebhookIsReady, metav1.ConditionTrue)
	return false, nil
}

func (m *SKRChartManager) RemoveWebhookChart(ctx context.Context, kyma *v1alpha1.Kyma,
	remoteClientCache *remote.ClientCache, kcpClient client.Client,
) error {
	skrClient, err := remote.NewRemoteClient(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, remoteClientCache)
	if err != nil {
		return err
	}
	skrCfg, err := remote.GetRemoteRestConfig(ctx, kcpClient, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy)
	if err != nil {
		return err
	}
	argsVals, err := m.generateHelmChartArgs(ctx, kcpClient)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(m.WebhookChartPath, ReleaseName, skrCfg, skrClient, argsVals)
	return installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeUninstall)
}

func (m *SKRChartManager) generateHelmChartArgs(ctx context.Context, kcpClient client.Client) (map[string]interface{}, error) {
	watcherList := &v1alpha1.WatcherList{}
	if err := kcpClient.List(ctx, watcherList, &client.ListOptions{}); err != nil {
		return nil, fmt.Errorf("error listing watcher resources: %w", err)
	}
	if len(watcherList.Items) == 0 {
		return nil, errors.New("found 0 watcher resources, expected at least 1")
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
		"kcpAddr":               kcpAddr,
		"resourcesLimitsMemory": m.SkrWebhookMemoryLimits,
		"resourcesLimitsCPU":    m.SkrWebhookCPULimits,
		customConfigKey:         string(bytes),
	}, nil
}

func (m *SKRChartManager) resolveKcpAddr(ctx context.Context, kcpClient client.Client) (string, error) {
	if m.KcpAddr != "" {
		return m.KcpAddr, nil
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
	m.KcpAddr = net.JoinHostPort(externalIP, strconv.Itoa(int(port)))
	return m.KcpAddr, nil
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

func installOrRemoveChartOnSKR(ctx context.Context, deployInfo modulelib.InstallInfo, mode Mode,
) error {
	logger := logf.FromContext(ctx)
	if mode == ModeUninstall {
		uninstalled, err := modulelib.UninstallChart(&logger, deployInfo, nil)
		if err != nil {
			return fmt.Errorf("failed to uninstall webhook config: %w", err)
		}
		if !uninstalled {
			return ErrSKRWebhookWasNotRemoved
		}
		ready, err := modulelib.ConsistencyCheck(&logger, deployInfo, nil)
		if err != nil {
			return fmt.Errorf("failed to verify webhook resources: %w", err)
		}
		if ready {
			return ErrSKRWebhookWasNotRemoved
		}
		return nil
	}

	installed, err := modulelib.InstallChart(&logger, deployInfo, nil)
	if err != nil {
		return fmt.Errorf("failed to install webhook config: %w", err)
	}
	if !installed {
		return ErrSKRWebhookHasNotBeenInstalled
	}
	ready, err := modulelib.ConsistencyCheck(&logger, deployInfo, nil)
	if err != nil {
		return fmt.Errorf("failed to verify webhook resources: %w", err)
	}
	if !ready {
		return ErrSKRWebhookNotReady
	}
	return nil
}
