package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	"golang.org/x/sync/errgroup"
	istioclientapiv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

const (
	webhookTLSCfgNameTpl = "%s-webhook-tls"
	defaultBufferSize    = 2048
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
	//nolint:wrapcheck
	return errGrp.Wait()
}

func resolveKcpAddr(kcpClient client.Client, managerConfig *SkrWebhookManagerConfig) (string, error) {
	ctx := context.TODO()

	// Get public KCP DNS name and port from the Gateway
	gateway := &istioclientapiv1beta1.Gateway{}

	if err := kcpClient.Get(ctx, client.ObjectKey{
		Namespace: managerConfig.IstioGatewayNamespace,
		Name:      managerConfig.IstioGatewayName,
	}, gateway); err != nil {
		return "", fmt.Errorf("failed to get istio gateway %s: %w", managerConfig.IstioGatewayName, err)
	}

	if len(gateway.Spec.GetServers()) != 1 || len(gateway.Spec.GetServers()[0].GetHosts()) != 1 {
		return "", ErrGatewayHostWronglyConfigured
	}
	host := gateway.Spec.GetServers()[0].GetHosts()[0]
	port := gateway.Spec.GetServers()[0].GetPort().GetNumber()

	if managerConfig.LocalGatewayPortOverwrite != "" {
		return net.JoinHostPort(host, managerConfig.LocalGatewayPortOverwrite), nil
	}

	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func ResolveTLSCertName(kymaName string) string {
	return fmt.Sprintf(webhookTLSCfgNameTpl, kymaName)
}

func getRawManifestUnstructuredResources(rawManifestReader io.Reader) ([]*unstructured.Unstructured, error) {
	decoder := machineryaml.NewYAMLOrJSONDecoder(rawManifestReader, defaultBufferSize)
	var resources []*unstructured.Unstructured
	for {
		resource := &unstructured.Unstructured{}
		err := decoder.Decode(resource)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("failed to decode raw manifest to unstructured: %w", err)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		resources = append(resources, resource)
	}
	return resources, nil
}
