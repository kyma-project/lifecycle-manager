package watcher

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	if err := errGrp.Wait(); err != nil {
		return fmt.Errorf("failed to run resource operation: %w", err)
	}
	return nil
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
