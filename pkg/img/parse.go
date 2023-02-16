package img

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultRepoSubdirectory = "component-descriptors"

var (
	ErrAccessTypeNotSupported           = errors.New("access type not supported")
	ErrContextTypeNotSupported          = errors.New("context type not supported")
	ErrComponentNameMappingNotSupported = errors.New("componentNameMapping not supported")
)

func Parse(
	descriptor *ocm.ComponentDescriptor,
) (Layers, error) {
	ctx := descriptor.GetEffectiveRepositoryContext()
	if ctx == nil {
		return Layers{}, nil
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
			ctx.GetType(), ErrContextTypeNotSupported,
		)
	}
}

func parseLayersByName(repo *ocm.OCIRegistryRepository, descriptor *ocm.ComponentDescriptor) (Layers, error) {
	layers := Layers{}
	for _, resource := range descriptor.Resources {
		access := resource.Access
		var layerRepresentation LayerRepresentation
		switch access.GetType() {
		case ocm.LocalOCIBlobType:
			ociAccess := &ocm.LocalOCIBlobAccess{}
			if err := access.DecodeInto(ociAccess); err != nil {
				return nil, fmt.Errorf("error while decoding the access into OCIRegistryRepository: %w", err)
			}

			layerRef, err := getOCIRef(repo, descriptor, ociAccess.Digest, resource.Labels)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
			}
			layerRepresentation = layerRef
		case ocmextensions.HelmChartRepositoryType:
			helmChartAccess := &ocmextensions.HelmChartRepositoryAccess{}
			if err := access.DecodeInto(helmChartAccess); err != nil {
				return nil, fmt.Errorf("error while decoding the access into OCIRegistryRepository: %w", err)
			}
			layerRepresentation = &Helm{
				ChartName: helmChartAccess.HelmChartName,
				URL:       helmChartAccess.HelmChartRepoURL,
				Version:   helmChartAccess.HelmChartVersion,
				Type:      HelmRepresentationType,
			}
		// this resource type is not relevant for module rendering but for security scanning only
		case ocm.OCIRegistryType:
			continue
		default:
			return nil, fmt.Errorf("error while parsing access type %s: %w",
				access.GetType(), ErrAccessTypeNotSupported,
			)
		}

		layers = append(layers, Layer{
			LayerName:           LayerName(resource.Name),
			LayerRepresentation: layerRepresentation,
		})
	}
	return layers, nil
}

func getOCIRef(repo *ocm.OCIRegistryRepository,
	descriptor *ocm.ComponentDescriptor,
	ref string,
	labels ocm.Labels,
) (*OCI, error) {
	layerRef := OCI{
		Type: OCIRepresentationType,
	}

	// if ref is not provided, we simply use the version of the descriptor, this will usually default
	// to a component version that is valid
	if ref == "" {
		layerRef.Ref = descriptor.GetVersion()
	} else {
		layerRef.Ref = ref
	}
	if registryCredValue, found := labels.Get(v1beta1.OCIRegistryCredLabel); found {
		credSecretLabel := make(map[string]string)
		if err := json.Unmarshal(registryCredValue, &credSecretLabel); err != nil {
			return nil, err
		}
		layerRef.CredSecretSelector = &metav1.LabelSelector{
			MatchLabels: credSecretLabel,
		}
	}

	switch repo.ComponentNameMapping {
	case ocm.OCIRegistryURLPathMapping:
		repoSubpath := DefaultRepoSubdirectory
		if ext, found := descriptor.GetLabels().Get(
			fmt.Sprintf("%s%s", ocm.OCIRegistryURLPathMapping, "RepoSubpath")); found {
			repoSubpath = string(ext)
		}
		layerRef.Repo = fmt.Sprintf("%s/%s", repo.BaseURL, repoSubpath)
		layerRef.Name = descriptor.GetName()
	case ocm.OCIRegistryDigestMapping:
		layerRef.Repo = repo.BaseURL
		layerRef.Name = sha256sum(descriptor.GetName())
	default:
		return nil, fmt.Errorf("error while parsing componentNameMapping %s: %w",
			repo.ComponentNameMapping, ErrComponentNameMappingNotSupported,
		)
	}
	return &layerRef, nil
}

func sha256sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
