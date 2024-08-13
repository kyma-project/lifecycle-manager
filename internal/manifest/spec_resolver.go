package manifest

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/auth"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/img"
)

type SpecResolver struct {
	kcpClient             client.Client
	manifestPathExtractor *img.PathExtractor
}

func NewSpecResolver(kcpClient client.Client, extractor *img.PathExtractor) *SpecResolver {
	return &SpecResolver{
		kcpClient:             kcpClient,
		manifestPathExtractor: extractor,
	}
}

var errRenderModeInvalid = errors.New("render mode is invalid")

func (s *SpecResolver) GetSpec(ctx context.Context, manifest *v1beta2.Manifest) (*declarativev2.Spec, error) {
	var imageSpec v1beta2.ImageSpec
	if err := yaml.Unmarshal(manifest.Spec.Install.Source.Raw, &imageSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if imageSpec.Type != v1beta2.OciRefType && imageSpec.Type != v1beta2.OciDirType {
		return nil, fmt.Errorf("could not determine render mode for %s: %w",
			client.ObjectKeyFromObject(manifest), errRenderModeInvalid)
	}

	keyChain, err := auth.LookupKeyChain(ctx, imageSpec, s.kcpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keyChain: %w", err)
	}

	rawManifestPath, err := s.manifestPathExtractor.FetchLayerToFile(ctx, imageSpec, keyChain)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw manifest from layer digest: %w", err)
	}

	return &declarativev2.Spec{
		ManifestName: manifest.Spec.Install.Name,
		Path:         rawManifestPath,
		OCIRef:       imageSpec.Ref,
	}, nil
}
