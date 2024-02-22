package manifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

// RawManifestInfo defines raw manifest information.
type RawManifestInfo struct {
	Path   string
	OCIRef string
}

type SpecResolver struct {
	KCP          *declarativev2.ClusterInfo
	ChartCache   string
	cachedCharts map[string]string
}

func NewSpecResolver(kcp *declarativev2.ClusterInfo) *SpecResolver {
	return &SpecResolver{
		KCP:          kcp,
		ChartCache:   os.TempDir(),
		cachedCharts: make(map[string]string),
	}
}

var (
	ErrRenderModeInvalid                   = errors.New("render mode is invalid")
	ErrInvalidObjectPassedToSpecResolution = errors.New("invalid object passed to spec resolution")
)

func (m *SpecResolver) Spec(ctx context.Context, obj declarativev2.Object) (*declarativev2.Spec, error) {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil, fmt.Errorf(
			"invalid type %s: %w", reflect.TypeOf(obj),
			ErrInvalidObjectPassedToSpecResolution,
		)
	}

	var imageSpec v1beta2.ImageSpec
	if err := yaml.Unmarshal(manifest.Spec.Install.Source.Raw, &imageSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	var mode declarativev2.RenderMode
	switch imageSpec.Type {
	case v1beta2.OciRefType:
		mode = declarativev2.RenderModeRaw
	default:
		return nil, fmt.Errorf("could not determine render mode for %s: %w",
			client.ObjectKeyFromObject(manifest), ErrRenderModeInvalid)
	}

	rawManifestInfo, err := m.getRawManifestForInstall(ctx, imageSpec, m.KCP.Client)
	if err != nil {
		return nil, err
	}

	return &declarativev2.Spec{
		ManifestName: manifest.Spec.Install.Name,
		Path:         rawManifestInfo.Path,
		OCIRef:       rawManifestInfo.OCIRef,
		Mode:         mode,
	}, nil
}

func (m *SpecResolver) getRawManifestForInstall(ctx context.Context,
	imageSpec v1beta2.ImageSpec,
	targetClient client.Client,
) (*RawManifestInfo, error) {
	keyChain, err := m.lookupKeyChain(ctx, imageSpec, targetClient)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch keyChain: %w", err)
	}

	// extract raw manifest from layer digest
	rawManifestPath, err := GetPathFromRawManifest(ctx, imageSpec, keyChain)
	if err != nil {
		return nil, fmt.Errorf("failed to extract raw manifest from layer digest: %w", err)
	}
	return &RawManifestInfo{
		Path:   rawManifestPath,
		OCIRef: imageSpec.Ref,
	}, nil
}

func (m *SpecResolver) lookupKeyChain(
	ctx context.Context, imageSpec v1beta2.ImageSpec, targetClient client.Client,
) (authn.Keychain, error) {
	var keyChain authn.Keychain
	var err error
	if imageSpec.CredSecretSelector != nil {
		if keyChain, err = ocmextensions.GetAuthnKeychain(ctx, imageSpec.CredSecretSelector, targetClient); err != nil {
			return nil, err
		}
	} else {
		keyChain = authn.DefaultKeychain
	}
	return authn.NewMultiKeychain(google.Keychain, keyChain), nil
}
