package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"golang.org/x/sync/errgroup"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO PKI move consts into other file if they are not needed here.
const (
	webhookTLSCfgNameTpl       = "%s-webhook-tls"
	SkrTLSName                 = "skr-webhook-tls"
	SkrResourceName            = "skr-webhook"
	defaultK3dLocalhostMapping = "host.k3d.internal"
	defaultBufferSize          = 2048
	skrChartFieldOwner         = client.FieldOwner(v1beta2.OperatorName)
	version                    = "v1"
	webhookTimeOutInSeconds    = 15
)

var ErrGatewayHostWronglyConfigured = errors.New("gateway should have configured exactly one server and one host")

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
		return net.JoinHostPort(defaultK3dLocalhostMapping, strconv.Itoa(managerConfig.LocalGatewayHTTPPortMapping)),
			nil
	}
	ctx := context.TODO()

	// Get public KCP DNS name and port from the Gateway
	gateway := &istiov1beta1.Gateway{}
	controlPlaneClient, err := client.New(kcpConfig, client.Options{})
	if err != nil {
		return "", err
	}
	err = controlPlaneClient.Get(ctx, client.ObjectKey{
		Namespace: managerConfig.IstioGatewayNamespace,
		Name:      managerConfig.IstioGatewayName,
	}, gateway)
	if err != nil {
		return "", err
	}

	if len(gateway.Spec.Servers) != 1 || len(gateway.Spec.Servers[0].Hosts) != 1 {
		return "", ErrGatewayHostWronglyConfigured
	}
	host := gateway.Spec.Servers[0].Hosts[0]
	port := gateway.Spec.Servers[0].Port.Number

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
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
