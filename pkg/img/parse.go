package img

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	_ "github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localociblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"github.com/open-component-model/ocm/pkg/runtime"

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
	descriptor *compdesc.ComponentDescriptor,
) (Layers, error) {
	ctx := descriptor.GetEffectiveRepositoryContext()
	if ctx == nil {
		return Layers{}, nil
	}
	return parseDescriptor(ctx, descriptor)
}

func parseDescriptor(ctx *runtime.UnstructuredTypedObject, descriptor *compdesc.ComponentDescriptor) (Layers, error) {
	repo, err := ctx.Evaluate(cpi.DefaultContext().RepositoryTypes())
	if err != nil {
		return nil, fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
	}
	typedRepo := repo.(*genericocireg.RepositorySpec)
	layersByName, err := parseLayersByName(typedRepo, descriptor)
	if err != nil {
		return nil, err
	}

	return layersByName, nil
}

func parseLayersByName(repo *genericocireg.RepositorySpec, descriptor *compdesc.ComponentDescriptor) (Layers, error) {
	layers := Layers{}
	for _, resource := range descriptor.Resources {
		access := resource.Access
		var layerRepresentation LayerRepresentation
		spec, err := ocm.DefaultContext().AccessSpecForSpec(access)
		if err != nil {
			return nil, err
		}
		switch access.GetType() {
		case localblob.TypeV1:
			fallthrough
		case localblob.Type:
			accessSpec := spec.(*localblob.AccessSpec)
			layerRef, err := getOCIRef(repo, descriptor, accessSpec.LocalReference, resource.Labels)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
			}
			layerRepresentation = layerRef
		case localociblob.TypeV1:
			fallthrough
		case localociblob.Type:
			accessSpec := spec.(*localociblob.AccessSpec)
			layerRef, err := getOCIRef(repo, descriptor, accessSpec.Digest.String(), resource.Labels)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
			}
			layerRepresentation = layerRef
		case ocmextensions.HelmChartRepositoryType:
			accessSpec := spec.(*ocmextensions.HelmChartRepositoryAccess)
			layerRepresentation = &Helm{
				ChartName: accessSpec.HelmChartName,
				URL:       accessSpec.HelmChartRepoURL,
				Version:   accessSpec.HelmChartVersion,
				Type:      HelmRepresentationType,
			}
		// this resource type is not relevant for module rendering but for security scanning only
		case ociartifact.Type:
			fallthrough
		case ociartifact.TypeV1:
			fallthrough
		case ociartifact.LegacyTypeV1:
			fallthrough
		case ociartifact.LegacyType:
			continue
		default:
			return nil, fmt.Errorf(
				"error while parsing access type %s: %w",
				access.GetType(), ErrAccessTypeNotSupported,
			)
		}

		layers = append(
			layers, Layer{
				LayerName:           LayerName(resource.Name),
				LayerRepresentation: layerRepresentation,
			},
		)
	}
	return layers, nil
}

func getOCIRef(
	repo *genericocireg.RepositorySpec,
	descriptor *compdesc.ComponentDescriptor,
	ref string,
	labels ocmmetav1.Labels,
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
	case genericocireg.OCIRegistryURLPathMapping:
		repoSubpath := DefaultRepoSubdirectory
		if ext, found := descriptor.GetLabels().Get(
			fmt.Sprintf("%s%s", genericocireg.OCIRegistryURLPathMapping, "RepoSubpath"),
		); found {
			repoSubpath = string(ext)
		}

		layerRef.Repo = fmt.Sprintf("%s/%s", repo.UniformRepositorySpec().RepositoryRef(), repoSubpath)
		layerRef.Name = descriptor.GetName()
	case genericocireg.OCIRegistryDigestMapping:
		layerRef.Repo = repo.UniformRepositorySpec().RepositoryRef()
		layerRef.Name = sha256sum(descriptor.GetName())
	default:
		return nil, fmt.Errorf(
			"error while parsing componentNameMapping %s: %w",
			repo.ComponentNameMapping, ErrComponentNameMappingNotSupported,
		)
	}
	return &layerRef, nil
}

func sha256sum(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
