package img

import (
	"errors"
	"fmt"

	"github.com/kyma-project/kyma-operator/operator/pkg/ocmextensions"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
)

const DefaultRepoSubdirectory = "component-descriptors"

var (
	ErrAccessTypeNotSupported           = errors.New("access type not supported")
	ErrContextTypeNotSupported          = errors.New("context type not supported")
	ErrComponentNameMappingNotSupported = errors.New("componentNameMapping not supported")
)

type SignatureVerification func(descriptor *ocm.ComponentDescriptor) error

var NoSignatureVerification SignatureVerification = func(descriptor *ocm.ComponentDescriptor) error { return nil } //nolint:lll,gochecknoglobals

func VerifyAndParse(
	descriptor *ocm.ComponentDescriptor, signatureVerification SignatureVerification,
) (Layers, error) {
	ctx := descriptor.GetEffectiveRepositoryContext()

	if err := signatureVerification(descriptor); err != nil {
		return nil, fmt.Errorf("signature verification error, untrusted: %w", err)
	}

	return parseDescriptor(ctx, descriptor)
}

func parseDescriptor(ctx *ocm.UnstructuredTypedObject, descriptor *ocm.ComponentDescriptor) (Layers, error) {
	switch ctx.GetType() {
	case ocm.OCIRegistryType:
		repo := &ocm.OCIRegistryRepository{}
		if err := ctx.DecodeInto(repo); err != nil {
			return nil, fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
		}

		layersByName, err := parseLayersByName(repo, descriptor)
		if err != nil {
			return nil, err
		}

		return layersByName, nil
	default:
		return nil, fmt.Errorf("error while parsing context type %s: %w",
			ctx.GetType(), ErrContextTypeNotSupported)
	}
}

func parseLayersByName(repo *ocm.OCIRegistryRepository, descriptor *ocm.ComponentDescriptor) (Layers, error) {
	layers := make(Layers)
	for _, resource := range descriptor.Resources {
		access := resource.Access
		var layerRepresentation LayerRepresentation
		switch access.GetType() {
		case ocm.LocalOCIBlobType:
			ociAccess := &ocm.LocalOCIBlobAccess{}
			if err := access.DecodeInto(ociAccess); err != nil {
				return nil, fmt.Errorf("error while decoding the access into OCIRegistryRepository: %w", err)
			}
			layerRef, err := getOCIRef(repo, descriptor, ociAccess.Digest)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
			}
			layerRepresentation = layerRef
		case ocmextensions.HelmChartRepositoryType:
			helmChartAccess := &ocmextensions.HelmChartRepositoryAccess{}
			if err := access.DecodeInto(helmChartAccess); err != nil {
				return nil, fmt.Errorf("error while decoding the access into OCIRegistryRepository: %w", err)
			}
			layerRepresentation = &HelmRef{
				ChartName: helmChartAccess.HelmChartName,
				URL:       helmChartAccess.HelmChartRepoURL,
				Version:   helmChartAccess.HelmChartVersion,
			}
		default:
			return nil, fmt.Errorf("error while parsing access type %s: %w",
				access.GetType(), ErrAccessTypeNotSupported)
		}

		layers[LayerName(resource.Name)] = Layer{
			LayerRepresentation: layerRepresentation,
			LayerType:           LayerType(resource.GetType()),
		}
	}
	return layers, nil
}

func getOCIRef(repo *ocm.OCIRegistryRepository, descriptor *ocm.ComponentDescriptor, ref string) (*OCIRef, error) {
	layerRef := OCIRef{
		Repo: repo.BaseURL,
	}
	switch repo.ComponentNameMapping { //nolint:exhaustive
	case ocm.OCIRegistryURLPathMapping:
		repoSubpath := DefaultRepoSubdirectory
		if ext, found := descriptor.GetLabels().Get(
			fmt.Sprintf("%s%s", ocm.OCIRegistryURLPathMapping, "RepoSubpath")); found {
			repoSubpath = string(ext)
		}
		layerRef.Repo = fmt.Sprintf("%s/%s", repo.BaseURL, repoSubpath)
		layerRef.Module = descriptor.GetName()
		// if ref is not provided, we simply use the version of the descriptor, this will usually default
		// to a component version that is valid
		if ref == "" {
			layerRef.Digest = descriptor.GetVersion()
		} else {
			layerRef.Digest = ref
		}
	default:
		return nil, fmt.Errorf("error while parsing componentNameMapping %s: %w",
			repo.ComponentNameMapping, ErrComponentNameMappingNotSupported)
	}
	return &layerRef, nil
}
