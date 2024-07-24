package manifest

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/img"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

type SpecResolver struct {
	kcpClient             client.Client
	manifestPathExtractor *PathExtractor
}

func NewSpecResolver(kcpClient client.Client, extractor *PathExtractor) *SpecResolver {
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

	keyChain, err := LookupKeyChain(ctx, imageSpec, s.kcpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keyChain: %w", err)
	}

	rawManifestPath, err := s.manifestPathExtractor.FetchLayerToFile(ctx, imageSpec, keyChain,
		string(img.RawManifestLayer))
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw manifest from layer digest: %w", err)
	}

	return &declarativev2.Spec{
		ManifestName: manifest.Spec.Install.Name,
		Path:         rawManifestPath,
		OCIRef:       imageSpec.Ref,
	}, nil
}

func LookupKeyChain(ctx context.Context, imageSpec v1beta2.ImageSpec,
	targetClient client.Client,
) (authn.Keychain, error) {
	var keyChain authn.Keychain
	var err error
	if imageSpec.CredSecretSelector == nil {
		keyChain = authn.DefaultKeychain
	} else if keyChain, err = ocmextensions.GetAuthnKeychain(ctx, imageSpec.CredSecretSelector, targetClient); err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(google.Keychain, keyChain), nil
}
