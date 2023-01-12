package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"
)

type Mode string

const (
	customConfigKey                = "modules"
	WebhookCfgAndDeploymentNameTpl = "%s-webhook"
	IstioSystemNs                  = "istio-system"
	IngressServiceName             = "istio-ingressgateway"
	releaseNameTpl                 = "%s-%s-skr"
	defaultK3dLocalhostMapping     = "host.k3d.internal"
)

var (
	ErrLoadBalancerIPIsNotAssigned = errors.New("load balancer service external ip is not assigned")
)

type SKRWebhookChartManager interface {
	Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error)
	Remove(ctx context.Context, kyma *v1alpha1.Kyma) error
}

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func generateWatchableConfigs(watchers []v1alpha1.Watcher) map[string]WatchableConfig {
	chartCfg := make(map[string]WatchableConfig, 0)
	for _, watcher := range watchers {
		statusOnly := watcher.Spec.Field == v1alpha1.StatusField
		chartCfg[watcher.GetModuleName()] = WatchableConfig{
			Labels:     watcher.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		}
	}
	return chartCfg
}

func ResolveSKRChartResourceName(resourceNameTpl string, kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(resourceNameTpl, skrChartReleaseName(kymaObjKey))
}

func skrChartReleaseName(kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(releaseNameTpl, kymaObjKey.Namespace, kymaObjKey.Name)
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

func resolveKcpAddr(kcpConfig *rest.Config, managerConfig *SkrChartManagerConfig) (string, error) {
	if managerConfig.WatcherLocalTestingEnabled {
		return net.JoinHostPort(defaultK3dLocalhostMapping, strconv.Itoa(managerConfig.GatewayHTTPPortMapping)), nil
	}
	// Get public KCP IP from the ISTIO load balancer external IP
	kcpClient, err := client.New(kcpConfig, client.Options{})
	if err != nil {
		return "", err
	}
	ctx := context.TODO()
	loadBalancerService := &corev1.Service{}
	if err := kcpClient.Get(ctx, client.ObjectKey{
		Name:      IngressServiceName,
		Namespace: IstioSystemNs,
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
