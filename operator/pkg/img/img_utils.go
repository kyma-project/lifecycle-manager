package img

import (
	"errors"
	"fmt"
	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/signatures"
	errwrap "github.com/pkg/errors"
)

const defaultSignatureName = "test-signature"

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

func ValidateAndParse(descriptor *v2.ComponentDescriptor, pathToVerificationKey string) (*Template, error) {
	ctx := descriptor.GetEffectiveRepositoryContext()

	sig, err := signatures.GetSignatureByName(descriptor, defaultSignatureName)
	if err != nil {
		return nil, errwrap.Wrapf(err, "error while reading RSA Verification for Parser at %s", ".path")
	}

	verifier, err := signatures.CreateRSAVerifierFromKeyFile(pathToVerificationKey)
	if err != nil {
		return nil, errwrap.Wrapf(err, "error while reading RSA Verification for Parser at %s", ".path")
	}

	verifyErr := verifier.Verify(*descriptor, *sig)
	if verifyErr != nil {
		return nil, errwrap.Wrapf(verifyErr, "RSA Verification Error! Untrusted!")
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
		return nil, errors.New(fmt.Sprintf("context type %s not supported", ctx.GetType()))
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
			layerRef, err := getLayerRef(repo, descriptor, resource.Digest.Value)
			if err != nil {
				return nil, errwrap.Wrap(err, "building the digest url")
			}
			layers[LayerName(resource.Name)] = Layer{
				LayerRef:  layerRef,
				LayerType: LayerType(access.GetType()),
			}
		default:
			return nil, errors.New(fmt.Sprintf("access type %s not supported", access.GetType()))
		}
	}
	return layers, nil
}

func getLayerRef(repo *v2.OCIRegistryRepository, descriptor *v2.ComponentDescriptor, ref string) (LayerRef, error) {
	layerRef := LayerRef{
		Repo: repo.BaseURL,
	}
	switch repo.ComponentNameMapping {
	case v2.OCIRegistryURLPathMapping:
		layerRef.Repo = fmt.Sprintf("%s/component-descriptors")
		layerRef.Module = descriptor.GetName()
		// if ref is not provided, we simply use the version of the descriptor, this will usually default
		// to a component version that is valid
		if ref == "" {
			layerRef.Digest = descriptor.GetVersion()
		} else {
			layerRef.Digest = ref
		}
	default:
		return layerRef, errors.New(fmt.Sprintf("unrecognized componentNameMapping %s", repo.ComponentNameMapping))
	}
	return layerRef, nil
}
