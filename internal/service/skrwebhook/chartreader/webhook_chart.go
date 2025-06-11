package chartreader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultBufferSize      = 2048
	rawManifestFilePathTpl = "%s/resources.yaml"
)

type Service struct {
	manifestFilePath string
}

func NewService(skrWebhookResourcePath string) *Service {
	manifestFilePath := fmt.Sprintf(rawManifestFilePathTpl, skrWebhookResourcePath)
	return &Service{
		manifestFilePath: manifestFilePath,
	}
}

type ResourceOperation func(ctx context.Context, clt client.Client, resource client.Object) error

// RunResourceOperationWithGroupedErrors loops through the resources and runs the passed operation
// on each resource concurrently and groups their returned errors into one.
func (s *Service) RunResourceOperationWithGroupedErrors(ctx context.Context, skrClient client.Client,
	resources []client.Object, operation ResourceOperation,
) error {
	errGrp, grpCtx := errgroup.WithContext(ctx)
	for idx := range resources {
		resIdx := idx
		errGrp.Go(func() error {
			return operation(grpCtx, skrClient, resources[resIdx])
		})
	}
	if err := errGrp.Wait(); err != nil {
		return fmt.Errorf("failed to run resource operation: %w", err)
	}
	return nil
}

func (s *Service) GetRawManifestUnstructuredResources() ([]*unstructured.Unstructured,
	error,
) {
	rawManifestReader, err := os.Open(s.manifestFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file path: %w", err)
	}
	defer rawManifestReader.Close()
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
