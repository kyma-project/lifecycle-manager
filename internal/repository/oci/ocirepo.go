package oci

import (
	"fmt"
	"regexp"

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

func (s *Repository) GetComponentDescriptor(
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
