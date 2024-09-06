package img

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localociblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"github.com/open-component-model/ocm/pkg/mime"
	"github.com/open-component-model/ocm/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

const DefaultRepoSubdirectory = "component-descriptors"

var (
	ErrAccessTypeNotSupported           = errors.New("access type not supported")
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
	repo, err := cpi.DefaultContext().RepositoryTypes().Convert(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
	}
	typedRepo, ok := repo.(*genericocireg.RepositorySpec)
	if !ok {
		return nil, common.ErrTypeAssert
	}
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
			return nil, fmt.Errorf("failed to create spec for acccess: %w", err)
		}
		switch access.GetType() {
		case localblob.TypeV1:
			fallthrough
		case localociblob.TypeV1:
			fallthrough
		case localociblob.Type:
			fallthrough
		case localblob.Type:
			accessSpec, ok := spec.(*localblob.AccessSpec)
			if !ok {
				return nil, common.ErrTypeAssert
			}
			layerRef, err := getOCIRef(repo, descriptor, accessSpec, resource.Labels)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
			}
			layerRepresentation = layerRef
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
				LayerName:           v1beta2.LayerName(resource.Name),
				LayerRepresentation: layerRepresentation,
			},
		)
	}
	return layers, nil
}

func getOCIRef(
	repo *genericocireg.RepositorySpec,
	descriptor *compdesc.ComponentDescriptor,
	accessSpec *localblob.AccessSpec,
	labels ocmmetav1.Labels,
) (*OCI, error) {
	layerRef := OCI{}
	if accessSpec.MediaType == mime.MIME_TAR {
		layerRef.Type = string(v1beta2.OciDirType)
	} else {
		layerRef.Type = string(v1beta2.OciRefType)
	}

	// if ref is not provided, we simply use the version of the descriptor, this will usually default
	// to a component version that is valid
	if accessSpec.LocalReference == "" {
		layerRef.Ref = descriptor.GetVersion()
	} else {
		layerRef.Ref = accessSpec.LocalReference
	}
	if registryCredValue, found := labels.Get(shared.OCIRegistryCredLabel); found {
		credSecretSelector, err := ocmextensions.GenerateLabelSelector(registryCredValue)
		if err != nil {
			return nil, err
		}
		layerRef.CredSecretSelector = credSecretSelector
	}

	switch repo.ComponentNameMapping {
	case genericocireg.OCIRegistryURLPathMapping:
		repoSubpath := DefaultRepoSubdirectory
		baseURL := repo.Name()
		if repo.SubPath != "" {
			baseURL = fmt.Sprintf("%s/%s", repo.Name(), repo.SubPath)
		}

		layerRef.Repo = fmt.Sprintf("%s/%s", baseURL, repoSubpath)
		layerRef.Name = descriptor.GetName()
	case genericocireg.OCIRegistryDigestMapping:
		baseURL := repo.Name()
		if repo.SubPath != "" {
			baseURL = fmt.Sprintf("%s/%s", repo.Name(), repo.SubPath)
		}
		layerRef.Repo = baseURL + "/"
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
