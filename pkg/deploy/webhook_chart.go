package deploy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/slok/go-helm-template/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	WebhookTLSCfgNameTpl           = "tls-watcher-%s"
	IstioSystemNs                  = "istio-system"
	IngressServiceName             = "istio-ingressgateway"
	releaseNameTpl                 = "%s-%s-skr"
	defaultK3dLocalhostMapping     = "host.k3d.internal"
	defaultBufferSize              = 2048
	caCertKey                      = "ca.crt"
	tlsCertKey                     = "tls.crt"
	tlsPrivateKeyKey               = "tls.key"
	skrChartFieldOwner             = client.FieldOwner("lifecycle-manager")
)

var ErrLoadBalancerIPIsNotAssigned = errors.New("load balancer service external ip is not assigned")

type SKRWebhookChartManager interface {
	// Install installs the watcher's webhook chart resources on the SKR cluster
	Install(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error)
	// Remove removes the watcher's webhook chart resources from the SKR cluster
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

type resourceOperation func(ctx context.Context, clt client.Client, resource *unstructured.Unstructured) error

// runResourceOperationWithGroupedErrors loops through the resources and runs the passed operation
// on each resource concurrently and groups their returned errors into one.
func runResourceOperationWithGroupedErrors(ctx context.Context, clt client.Client,
	resources []*unstructured.Unstructured, operation resourceOperation,
) error {
	errGrp, grpCtx := errgroup.WithContext(ctx)
	for idx := range resources {
		resIdx := idx
		errGrp.Go(func() error {
			return operation(grpCtx, clt, resources[resIdx])
		})
	}
	return errGrp.Wait()
}

func skrChartReleaseName(kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(releaseNameTpl, kymaObjKey.Namespace, kymaObjKey.Name)
}

func generateHelmChartArgs(ctx context.Context, kcpClient client.Client, kymaObjKey client.ObjectKey,
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

	tlsSecret := &corev1.Secret{}
	secretObjKey := client.ObjectKey{
		Namespace: kymaObjKey.Namespace,
		Name:      ResolveSKRChartResourceName(WebhookTLSCfgNameTpl, kymaObjKey),
	}

	if err := kcpClient.Get(ctx, secretObjKey, tlsSecret); err != nil {
		return nil, fmt.Errorf("error fetching TLS secret: %w", err)
	}

	return map[string]interface{}{
		"caCert": string(tlsSecret.Data[caCertKey]),
		"tls": map[string]string{
			"cert":          string(tlsSecret.Data[tlsCertKey]),
			"privateKey":    string(tlsSecret.Data[tlsPrivateKeyKey]),
			"secretResVer":  tlsSecret.GetResourceVersion(),
			"webhookServer": "true",
			"callback":      "false",
		},
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

func renderChartToRawManifest(ctx context.Context, kymaObjKey client.ObjectKey,
	chartPath string, chartArgValues map[string]interface{},
) (string, error) {
	chartFS := os.DirFS(chartPath)
	chart, err := helm.LoadChart(ctx, chartFS)
	if err != nil {
		return "", nil
	}
	return helm.Template(ctx, helm.TemplateConfig{
		Chart:       chart,
		ReleaseName: skrChartReleaseName(kymaObjKey),
		Namespace:   metav1.NamespaceDefault,
		Values:      chartArgValues,
	})
}

// ResolveSKRChartResourceName resolves a resource name that belongs to the SKR webhook's Chart
// using the resource name's template.
func ResolveSKRChartResourceName(resourceNameTpl string, kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf(resourceNameTpl, skrChartReleaseName(kymaObjKey))
}
