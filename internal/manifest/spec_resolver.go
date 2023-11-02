package manifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

	"github.com/google/go-containerregistry/pkg/v1/google"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"

	"github.com/google/go-containerregistry/pkg/authn"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RawManifestInfo defines raw manifest information.
type RawManifestInfo struct {
	Path   string
	OCIRef string
}

type SpecResolver struct {
	KCP          *declarative.ClusterInfo
	ChartCache   string
	cachedCharts map[string]string
}

func NewSpecResolver(kcp *declarative.ClusterInfo) *SpecResolver {
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

func (m *SpecResolver) Spec(ctx context.Context, obj declarative.Object,
	remoteClient client.Client,
) (*declarative.Spec, error) {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil, fmt.Errorf(
			"invalid type %s: %w", reflect.TypeOf(obj),
			ErrInvalidObjectPassedToSpecResolution,
		)
	}

	targetClient := m.KCP.Client
	if manifest.Labels[v1beta2.IsRemoteModuleTemplate] == v1beta2.EnableLabelValue {
		targetClient = remoteClient
	}
	var imageSpec v1beta2.ImageSpec
	if err := yaml.Unmarshal(manifest.Spec.Install.Source.Raw, &imageSpec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	var mode declarative.RenderMode
	switch imageSpec.Type {
	case v1beta2.OciRefType:
		mode = declarative.RenderModeRaw
	default:
		return nil, fmt.Errorf("could not determine render mode for %s: %w",
			client.ObjectKeyFromObject(manifest), ErrRenderModeInvalid)
	}

	rawManifestInfo, err := m.getRawManifestForInstall(ctx, imageSpec, targetClient)
	if err != nil {
		return nil, err
	}

	return &declarative.Spec{
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
