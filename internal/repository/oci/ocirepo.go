package oci

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"ocm.software/ocm/api/credentials"
	"ocm.software/ocm/api/oci/extensions/repositories/ocireg"
	"ocm.software/ocm/api/ocm/compdesc"
	"ocm.software/ocm/api/ocm/cpi"
	"ocm.software/ocm/api/utils/runtime"
)

type CredResolverFunc func(ctx cpi.Context, userPasswordCreds, registryURL string) (credentials.Credentials, error)

// TODO: Initial structure: Rename, refactor
type Repository struct {
	insecure          bool
	userPasswordCreds string
	registryURL       string
	credResolver      CredResolverFunc
}

func NewRepository(
	registryURL string,
	userPasswordCreds string,
	insecure bool,
	credResolver CredResolverFunc,
) *Repository {
	return &Repository{
		insecure:          insecure,
		userPasswordCreds: userPasswordCreds,
		registryURL:       registryURL,
		credResolver:      credResolver,
	}
}

func (s *Repository) GetComponentDescriptor(name, version string) (*compdesc.ComponentDescriptor, error) {

	//{"componentDescriptorLayer":{"mediaType":"application/vnd.ocm.software.component-descriptor.v2+yaml+tar","digest":"sha256:4e51d8f80b88bdbd208e6e22314376a0d5212026bf3054f8ef79d43250e5182b","size":4608}}
	ref := fmt.Sprintf("k3d-kcp-registry.localhost:5000/component-descriptors/%s:%s", name, version)

	// Fetch the image config to get the ComponentDescriptor layer info
	configBytes, err := crane.Config(ref, crane.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to get config for ref=%q: %w", ref, err)
	}

	// Parse the config data to extract the ComponentDescriptor layer digest
	genericRepresentation := new(map[string]any)
	err = json.Unmarshal(configBytes, genericRepresentation)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config for ref=%q: %w", ref, err)
	}

	layerInfo, ok := (*genericRepresentation)["componentDescriptorLayer"]
	if !ok {
		return nil, fmt.Errorf("componentDescriptorLayer not found in config for ref=%q", ref)
	}

	layerInfoMap, ok := layerInfo.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid componentDescriptorLayer format in config for ref=%q", ref)
	}

	digest, ok := layerInfoMap["digest"].(string)
	if !ok {
		return nil, fmt.Errorf("digest not found or invalid in componentDescriptorLayer for ref=%q", ref)
	}

	layer, err := crane.PullLayer(fmt.Sprintf("%s@%s", ref, digest), crane.Insecure)
	if err != nil {
		return nil, fmt.Errorf("failed to pull layer for ref=%q with digest=%q: %w", ref, digest, err)
	}

	layerReader, err := layer.Uncompressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get uncompressed layer reader for ref=%q with digest=%q: %w", ref, digest, err)
	}
	defer layerReader.Close()

	layerBytes, err := io.ReadAll(layerReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read layer content for ref=%q with digest=%q: %w", ref, digest, err)
	}

	compdescBytes, err := extractDescriptor(layerBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to extract component descriptor from layer for ref=%q with digest=%q: %w", ref, digest, err)
	}

	cd, err := compdesc.Decode(compdescBytes)
	if err != nil {
		fmt.Println(strings.Repeat("=", 80))
		fmt.Println(string(compdescBytes))
		fmt.Println(strings.Repeat("=", 80))
		return nil, fmt.Errorf("failed to decode component descriptor from layer for ref=%q with digest=%q: %w", ref, digest, err)
	}

	return cd, nil
}

func (s *Repository) GetComponentDescriptorOld(
	name, version string,
) (*compdesc.ComponentDescriptor, error) {
	repo, err := s.getRepository()
	if err != nil {
		return nil, fmt.Errorf("could not get repository: %w", err)
	}

	cva, err := repo.LookupComponentVersion(name, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get ComponentVersion for name=%q version=%q: %w", name, version, err)
	}

	return cva.GetDescriptor(), nil
}

func (s *Repository) getRepository() (cpi.Repository, error) {
	ctx := cpi.DefaultContext()

	// TODO: This should be one-time setup, not per call
	creds, err := s.credResolver(ctx, s.userPasswordCreds, s.registryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve credentials: %w", err)
	}

	ociRepoSpec := &ocireg.RepositorySpec{
		ObjectVersionedType: runtime.NewVersionedObjectType(ocireg.Type),
		BaseURL:             ConstructRegistryUrl(s.registryURL, s.insecure),
	}

	ociRepo, err := ctx.RepositoryTypes().Convert(ociRepoSpec)
	if err != nil {
		return nil, fmt.Errorf("could not convert repository spec: %w", err)
	}

	repo, err := ctx.RepositoryForSpec(ociRepo, creds)
	if err != nil {
		return nil, fmt.Errorf("could not create repository from spec: %w", err)
	}

	return repo, nil
}

func ConstructRegistryUrl(url string, insecure bool) string {
	registryURL := noSchemeURL(url)
	if insecure {
		registryURL = "http://" + registryURL
	}

	return registryURL
}

func noSchemeURL(url string) string {
	regex := regexp.MustCompile(`^https?://`)
	return regex.ReplaceAllString(url, "")
}

func extractDescriptor(layerBytes []byte) ([]byte, error) {
	tr := tar.NewReader(bytes.NewReader(layerBytes))

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // end of archive
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == "component-descriptor.yaml" {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return buf.Bytes(), nil
		}
	}

	return nil, fmt.Errorf("component-descriptor.yaml not found")
}
