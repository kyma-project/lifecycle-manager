package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
)

// TODO PKI move consts into other file if they are not needed here.
const (
	webhookTLSCfgNameTpl         = "%s-webhook-tls"
	SkrTLSName                   = "skr-webhook-tls"
	SkrResourceName              = "skr-webhook"
	IstioSystemNs                = "istio-system"
	IngressServiceName           = "istio-ingressgateway"
	defaultK3dLocalhostMapping   = "host.k3d.internal"
	defaultBufferSize            = 2048
	skrChartFieldOwner           = client.FieldOwner(v1beta1.OperatorName)
	version                      = "v1"
	webhookTimeOutInSeconds      = 15
	allResourcesWebhookRule      = "*"
	statusSubResourceWebhookRule = "*/status"
)

var ErrLoadBalancerIPIsNotAssigned = errors.New("load balancer service external ip is not assigned")

type WatchableConfig struct {
	Labels     map[string]string `json:"labels"`
	StatusOnly bool              `json:"statusOnly"`
}

func generateWatchableConfigs(watchers []v1beta1.Watcher) map[string]WatchableConfig {
	chartCfg := make(map[string]WatchableConfig, 0)
	for _, watcher := range watchers {
		statusOnly := watcher.Spec.Field == v1beta1.StatusField
		chartCfg[watcher.GetModuleName()] = WatchableConfig{
			Labels:     watcher.Spec.LabelsToWatch,
			StatusOnly: statusOnly,
		}
	}
	return chartCfg
}

type resourceOperation func(ctx context.Context, clt client.Client, resource client.Object) error

// runResourceOperationWithGroupedErrors loops through the resources and runs the passed operation
// on each resource concurrently and groups their returned errors into one.
func runResourceOperationWithGroupedErrors(ctx context.Context, clt client.Client,
	resources []client.Object, operation resourceOperation,
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

func resolveKcpAddr(kcpConfig *rest.Config, managerConfig *SkrWebhookManagerConfig) (string, error) {
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

func resolveRemoteNamespace(kyma *v1beta1.Kyma) string {
	if kyma.Spec.Sync.Namespace != "" {
		return kyma.Spec.Sync.Namespace
	}
	return kyma.Namespace
}

func ResolveTLSCertName(kymaName string) string {
	return fmt.Sprintf(webhookTLSCfgNameTpl, kymaName)
}

func getRawManifestUnstructuredResources(rawManifestReader io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(rawManifestReader, defaultBufferSize)
	var resources []*unstructured.Unstructured
	for {
		resource := &unstructured.Unstructured{}
		err := decoder.Decode(resource)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		if errors.Is(err, io.EOF) {
			break
		}
		resources = append(resources, resource)
	}
	return resources, nil
}
