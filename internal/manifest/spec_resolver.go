package manifest

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

type KeyChainLookup interface {
	Get(ctx context.Context, imageSpec v1beta2.ImageSpec) (authn.Keychain, error)
}

type PathExtractor interface {
	GetPathFromRawManifest(ctx context.Context, imageSpec v1beta2.ImageSpec, keyChain authn.Keychain) (string, error)
}

type SpecResolver struct {
	keyChainLookup        KeyChainLookup
	manifestPathExtractor PathExtractor
}

func NewSpecResolver(kcLookup KeyChainLookup, extractor PathExtractor) *SpecResolver {
	return &SpecResolver{
		keyChainLookup:        kcLookup,
		manifestPathExtractor: extractor,
	}
}

var errRenderModeInvalid = errors.New("render mode is invalid")

func (s *SpecResolver) GetSpec(ctx context.Context, manifest *v1beta2.Manifest) (*declarativev2.Spec, error) {
	var imageSpec v1beta2.ImageSpec
	if err := yaml.Unmarshal(manifest.Spec.Install.Source.Raw, &imageSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	if imageSpec.Type != v1beta2.OciRefType {
		return nil, fmt.Errorf("could not determine render mode for %s: %w",
			client.ObjectKeyFromObject(manifest), errRenderModeInvalid)
	}

	keyChain, err := s.keyChainLookup.Get(ctx, imageSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keyChain: %w", err)
	}

	rawManifestPath, err := s.manifestPathExtractor.GetPathFromRawManifest(ctx, imageSpec, keyChain)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw manifest from layer digest: %w", err)
	}

	return &declarativev2.Spec{
		ManifestName: manifest.Spec.Install.Name,
		Path:         rawManifestPath,
		OCIRef:       imageSpec.Ref,
	}, nil
}
