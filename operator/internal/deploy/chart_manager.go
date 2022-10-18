package deploy

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type SKRChartManager struct {
	kcpClient              client.Client
	WebhookChartPath       string
	SkrWebhookMemoryLimits string
	SkrWebhookCPULimits    string
	KcpAddr                string
}

func NewSKRChartManager(ctx context.Context, kcpClient client.Client,
	chartPath, memoryLimits, cpuLimits string,
) (*SKRChartManager, error) {
	mgr := &SKRChartManager{
		kcpClient:              kcpClient,
		WebhookChartPath:       chartPath,
		SkrWebhookMemoryLimits: memoryLimits,
		SkrWebhookCPULimits:    cpuLimits,
	}
	kcpAddr, err := resolveKcpAddr(ctx, kcpClient)
	if err != nil {
		return nil, err
	}
	mgr.KcpAddr = kcpAddr
	return mgr, nil
}

func (m *SKRChartManager) InstallWebhookChart(ctx context.Context,
	watcherList *v1alpha1.WatcherList, kyma *v1alpha1.Kyma, inClusterCfg *rest.Config,
) error {
	skrCfg, err := m.getSKRClientFromKyma(ctx, kyma, inClusterCfg)
	if err != nil {
		return err
	}
	skrClient, err := client.New(skrCfg, client.Options{})
	if err != nil {
		return err
	}
	argsVals, err := m.generateHelmChartArgs(ctx, watcherList)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(m.WebhookChartPath, ReleaseName, skrCfg, skrClient, argsVals)
	return installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeInstall)
}

func (m *SKRChartManager) RemoveWebhookChart(ctx context.Context,
	watcherList *v1alpha1.WatcherList, kyma *v1alpha1.Kyma, inClusterCfg *rest.Config,
) error {
	skrCfg, err := m.getSKRClientFromKyma(ctx, kyma, inClusterCfg)
	if err != nil {
		return err
	}
	skrClient, err := client.New(skrCfg, client.Options{})
	if err != nil {
		return err
	}
	argsVals, err := m.generateHelmChartArgs(ctx, watcherList)
	if err != nil {
		return err
	}
	skrWatcherInstallInfo := prepareInstallInfo(m.WebhookChartPath, ReleaseName, skrCfg, skrClient, argsVals)
	return installOrRemoveChartOnSKR(ctx, skrWatcherInstallInfo, ModeUninstall)
}

func (m *SKRChartManager) getSKRClientFromKyma(ctx context.Context, kyma *v1alpha1.Kyma, kcpCfg *rest.Config,
) (*rest.Config, error) {

	if kyma.Spec.Sync.Strategy == v1alpha1.SyncStrategyLocalClient || !kyma.Spec.Sync.Enabled {
		return kcpCfg, nil
	}
	secret := &corev1.Secret{}
	//nolint:gosec
	err := m.kcpClient.Get(ctx, client.ObjectKeyFromObject(kyma), secret)
	if err != nil {
		return nil, err
	}
	restCfg, err := clientcmd.RESTConfigFromKubeConfig(secret.Data[kubeconfigKey])
	if err != nil {
		return nil, err
	}
	return restCfg, nil

}

func (m *SKRChartManager) generateHelmChartArgs(ctx context.Context,
	watcherList *v1alpha1.WatcherList,
) (map[string]interface{}, error) {
	if len(watcherList.Items) == 0 {
		return map[string]interface{}{
			"kcpAddr":               m.KcpAddr,
			"resourcesLimitsMemory": m.SkrWebhookMemoryLimits,
			"resourcesLimitsCPU":    m.SkrWebhookCPULimits,
			customConfigKey:         "",
		}, nil
	}
	chartCfg := generateWatchableConfigs(watcherList)
	bytes, err := k8syaml.Marshal(chartCfg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"kcpAddr":               m.KcpAddr,
		"resourcesLimitsMemory": m.SkrWebhookMemoryLimits,
		"resourcesLimitsCPU":    m.SkrWebhookCPULimits,
		customConfigKey:         string(bytes),
	}, nil
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
