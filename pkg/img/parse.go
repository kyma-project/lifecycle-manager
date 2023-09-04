package img

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/pkg/common"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/open-component-model/ocm/pkg/contexts/ocm"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/localociblob"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/accessmethods/ociartifact"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmv1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"github.com/open-component-model/ocm/pkg/runtime"
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
		case localblob.Type:
			accessSpec, ok := spec.(*localblob.AccessSpec)
			if !ok {
				return nil, common.ErrTypeAssert
			}
			layerRef, err := getOCIRef(repo, descriptor, accessSpec.LocalReference, resource.Labels)
			if err != nil {
				return nil, fmt.Errorf("building the digest url: %w", err)
			}
			layerRepresentation = layerRef
		case localociblob.TypeV1:
			fallthrough
		case localociblob.Type:
			return nil, fmt.Errorf("localOciBlob is no longer supported but was found as layer type, "+
				"use LocalBlob instead: %w", err)
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
	labels ocmv1.Labels,
) (*OCI, error) {
	layerRef := OCI{
		Type: string(v1beta2.OciRefType),
	}

	// if ref is not provided, we simply use the version of the descriptor, this will usually default
	// to a component version that is valid
	if ref == "" {
		layerRef.Ref = descriptor.GetVersion()
	} else {
		layerRef.Ref = ref
	}
	if registryCredValue, found := labels.Get(v1beta2.OCIRegistryCredLabel); found {
		credSecretSelector, err := ocmextensions.GenerateLabelSelector(registryCredValue)
		if err != nil {
			return nil, err
		}
		layerRef.CredSecretSelector = credSecretSelector
	}

	switch repo.ComponentNameMapping {
	case genericocireg.OCIRegistryURLPathMapping:
		repoSubpath := DefaultRepoSubdirectory
		if ext, found := descriptor.GetLabels().Get(
			fmt.Sprintf("%s%s", genericocireg.OCIRegistryURLPathMapping, "RepoSubpath"),
		); found {
			repoSubpath = string(ext)
		}

		layerRef.Repo = fmt.Sprintf("%s/%s", repo.Name(), repoSubpath)
		layerRef.Name = descriptor.GetName()
	case genericocireg.OCIRegistryDigestMapping:
		layerRef.Repo = fmt.Sprintf("%s/", repo.Name())
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
