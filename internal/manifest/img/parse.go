package img

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/cpi"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/localblob"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/localociblob"
	"ocm.software/ocm/api/ocm/extensions/accessmethods/ociartifact"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg"
	"ocm.software/ocm/api/ocm/extensions/repositories/genericocireg/componentmapping"
	"ocm.software/ocm/api/utils/mime"
	"ocm.software/ocm/api/utils/runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/common"
)

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
		var ociRef *OCI
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
			ociRef, err = getOCIRef(repo, descriptor, accessSpec)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
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
				LayerName:           v1beta2.LayerName(resource.Name),
				LayerRepresentation: ociRef,
			},
		)
	}
	return layers, nil
}

func getOCIRef(
	repo *genericocireg.RepositorySpec,
	descriptor *compdesc.ComponentDescriptor,
	accessSpec *localblob.AccessSpec,
) (*OCI, error) {
	layerRef := OCI{}
	if accessSpec.MediaType == mime.MIME_TAR || accessSpec.MediaType == mime.MIME_TGZ {
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

	switch repo.ComponentNameMapping {
	case genericocireg.OCIRegistryURLPathMapping:
		repoSubpath := componentmapping.ComponentDescriptorNamespace
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
