// Package render produces the target resources to apply for a Manifest by
// parsing the manifest layer and applying a configured chain of transforms.
package render

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

// Service renders the target resources for a Manifest. It owns the
// manifest parser cache and the ordered list of transforms that adapt the
// parsed resources for deployment to a SKR cluster.
type Service struct {
	parser     declarativev2.CachedManifestParser
	transforms []declarativev2.ResourceTransform
}

// NewService wires a Service from its dependencies. Transforms are applied
// in the order provided.
func NewService(
	parser declarativev2.CachedManifestParser,
	transforms []declarativev2.ResourceTransform,
) *Service {
	return &Service{parser: parser, transforms: transforms}
}

// RenderTargetResources returns the resources to be synced to the SKR for the
// given Manifest. It parses the manifest layer (using the parser cache) and
// applies the configured transforms to the parsed resources before returning
// them as client.Objects.
func (s *Service) RenderTargetResources(ctx context.Context, skrClient skrclient.Client,
	manifest *v1beta2.Manifest, spec *declarativev2.Spec,
) ([]client.Object, error) {
	parsed, err := s.parser.Parse(spec)
	if err != nil {
		return nil, err
	}

	for _, transform := range s.transforms {
		if err := transform(ctx, skrClient, manifest, parsed.Items); err != nil {
			return nil, err
		}
	}

	result := make([]client.Object, 0, len(parsed.Items))
	for _, unstrObj := range parsed.Items {
		result = append(result, unstrObj)
	}
	return result, nil
}

// EvictCache removes the cached parse result for the given spec, forcing the
// next RenderTargetResources call to re-parse the manifest from disk.
func (s *Service) EvictCache(spec *declarativev2.Spec) {
	s.parser.EvictCache(spec)
}
