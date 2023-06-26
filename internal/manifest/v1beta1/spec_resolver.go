package v1beta1

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"

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
	Name   string
}

type ManifestSpecResolver struct {
	*v1beta2.Codec

	ChartCache   string
	cachedCharts map[string]string
}

func NewManifestSpecResolver(codec *v1beta2.Codec) *ManifestSpecResolver {
	return &ManifestSpecResolver{
		Codec:        codec,
		ChartCache:   os.TempDir(),
		cachedCharts: make(map[string]string),
	}
}

var (
	ErrRenderModeInvalid                   = errors.New("render mode is invalid")
	ErrInvalidObjectPassedToSpecResolution = errors.New("invalid object passed to spec resolution")
)

func (m *ManifestSpecResolver) Spec(ctx context.Context, obj declarative.Object,
	targetClient declarative.Client) (*declarative.Spec, error) {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil, fmt.Errorf(
			"invalid type %s: %w", reflect.TypeOf(obj),
			ErrInvalidObjectPassedToSpecResolution,
		)
	}

	specType, err := v1beta2.GetSpecType(manifest.Spec.Install.Source.Raw)
	if err != nil {
		return nil, err
	}

	keyChain, err := m.lookupKeyChain(ctx, manifest.Spec.Config, targetClient)
	if err != nil {
		return nil, err
	}

	rawManifestInfo, err := m.getRawManifestForInstall(ctx, manifest.Spec.Install, specType, keyChain)
	if err != nil {
		return nil, err
	}

	var mode declarative.RenderMode
	switch specType {
	case v1beta2.OciRefType:
		mode = declarative.RenderModeRaw
	case v1beta2.NilRefType:
		return nil, fmt.Errorf("could not determine render mode for %s: %w",
			client.ObjectKeyFromObject(manifest), ErrRenderModeInvalid)
	}

	return &declarative.Spec{
		ManifestName: manifest.Spec.Install.Name,
		Path:         rawManifestInfo.Path,
		OCIRef:       rawManifestInfo.OCIRef,
		Mode:         mode,
	}, nil
}

var (
	ErrUnsupportedInstallType = errors.New("install type is not supported")
	ErrEmptyInstallType       = errors.New("empty install type")
)

func (m *ManifestSpecResolver) getRawManifestForInstall(
	ctx context.Context,
	install v1beta2.InstallInfo,
	specType v1beta2.RefTypeMetadata,
	keyChain authn.Keychain,
) (*RawManifestInfo, error) {
	var err error
	switch specType {
	case v1beta2.OciRefType:
		var imageSpec v1beta2.ImageSpec
		if err = m.Codec.Decode(install.Source.Raw, &imageSpec, specType); err != nil {
			return nil, err
		}

		// extract raw manifest from layer digest
		rawManifestPath, err := GetPathFromRawManifest(ctx, imageSpec, keyChain)
		if err != nil {
			return nil, err
		}

		return &RawManifestInfo{
			Name:   install.Name,
			Path:   rawManifestPath,
			OCIRef: imageSpec.Ref,
		}, nil
	case v1beta2.NilRefType:
		return nil, ErrEmptyInstallType
	default:
		return nil, fmt.Errorf("%s is invalid: %w", specType, ErrUnsupportedInstallType)
	}
}

func (m *ManifestSpecResolver) lookupKeyChain(
	ctx context.Context, imageSpec v1beta2.ImageSpec, targetClient declarative.Client,
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
	return keyChain, nil
}
