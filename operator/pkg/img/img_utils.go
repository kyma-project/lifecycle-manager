package img

import (
	"errors"
	"fmt"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	errwrap "github.com/pkg/errors"
)

const DefaultRepoSubdirectory = "component-descriptors"

var (
	ErrAccessTypeNotSupported           = errors.New("access type not supported")
	ErrContextTypeNotSupported          = errors.New("context type not supported")
	ErrComponentNameMappingNotSupported = errors.New("componentNameMapping not supported")
)

type LayerName string

type LayerRef struct {
	Repo   string
	Module string
	Digest string
}

func (r LayerRef) String() string {
	return fmt.Sprintf("%s/%s:%s", r.Repo, r.Module, r.Digest)
}

type LayerType string

type Layer struct {
	LayerRef
	LayerType
}
type Layers map[LayerName]Layer

type Template struct {
	Layers
	ImageURL string
}

type SignatureVerification func(descriptor *v2.ComponentDescriptor) error

var NoSignatureVerification SignatureVerification = func(descriptor *v2.ComponentDescriptor) error { return nil } //nolint:lll,gochecknoglobals

func VerifyAndParse(
	descriptor *v2.ComponentDescriptor, signatureVerification SignatureVerification,
) (*Template, error) {
	ctx := descriptor.GetEffectiveRepositoryContext()

	if err := signatureVerification(descriptor); err != nil {
		return nil, errwrap.Wrapf(err, "Signature Verification Error! Untrusted!")
	}

	return parseDescriptor(ctx, descriptor)
}

func parseDescriptor(ctx *v2.UnstructuredTypedObject, descriptor *v2.ComponentDescriptor) (*Template, error) {
	switch ctx.GetType() {
	case v2.OCIRegistryType:
		repo := &v2.OCIRegistryRepository{}
		if err := ctx.DecodeInto(repo); err != nil {
			return nil, errwrap.Wrap(err, "error while decoding the repository context into an OCI registry")
		}

		imageURL, err := getLayerRef(repo, descriptor, "")
		if err != nil {
			return nil, errwrap.Wrap(err, "error while parsing the imageURL")
		}

		layersByName, err := parseLayersByName(repo, descriptor)
		if err != nil {
			return nil, err
		}

		return &Template{layersByName, imageURL.String()}, nil
	default:
		return nil, fmt.Errorf("error while parsing context type %s: %w",
			ctx.GetType(), ErrContextTypeNotSupported)
	}
}

func parseLayersByName(repo *v2.OCIRegistryRepository, descriptor *v2.ComponentDescriptor) (Layers, error) {
	layers := make(Layers)
	for _, resource := range descriptor.Resources {
		access := resource.Access
		switch access.GetType() {
		case v2.LocalOCIBlobType:
			ociAccess := &v2.LocalOCIBlobAccess{}
			if err := access.DecodeInto(ociAccess); err != nil {
				return nil, errwrap.Wrap(err, "error while decoding the access into OCIRegistryRepository")
			}
			layerRef, err := getLayerRef(repo, descriptor, ociAccess.Digest)
			if err != nil {
				return nil, errwrap.Wrap(err, "building the digest url")
			}
			layers[LayerName(resource.Name)] = Layer{
				LayerRef:  *layerRef,
				LayerType: LayerType(resource.GetType()),
			}
		default:
			return nil, fmt.Errorf("error while parsing access type %s: %w",
				access.GetType(), ErrAccessTypeNotSupported)
		}
	}
	return layers, nil
}

func getLayerRef(repo *v2.OCIRegistryRepository, descriptor *v2.ComponentDescriptor, ref string) (*LayerRef, error) {
	layerRef := LayerRef{
		Repo: repo.BaseURL,
	}
	switch repo.ComponentNameMapping { //nolint:exhaustive
	case v2.OCIRegistryURLPathMapping:
		repoSubpath := DefaultRepoSubdirectory
		if ext, found := descriptor.GetLabels().Get(fmt.Sprintf("%s%s", v2.OCIRegistryURLPathMapping, "RepoSubpath")); found {
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
