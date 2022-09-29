package deploy

import (
	"context"
	"fmt"

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
	ModeInstall   = Mode("install")
	ModeUninstall = Mode("uninstall")
)

func InstallSKRWebhook(ctx context.Context, chartPath, releaseName string,
	obj *v1alpha1.Watcher, restConfig *rest.Config,
) error {
	argsVals, err := generateHelmChartArgsForCR(obj)
	if err != nil {
		return err
	}
	restClient, err := client.New(restConfig, client.Options{})
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

func generateHelmChartArgsForCR(obj *v1alpha1.Watcher) (map[string]interface{}, error) {
	chartCfg := generateWatchableConfigForCR(obj)
	bytes, err := k8syaml.Marshal(chartCfg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
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
