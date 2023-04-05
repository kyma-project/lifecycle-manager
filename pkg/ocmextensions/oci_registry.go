package ocmextensions

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNoEffectiveRepositoryContext = errors.New("no effective repository context")
)

type OCIRegistry struct {
	target string
	host   string
}

func (o OCIRegistry) String() string {
	return o.target
}

func (o OCIRegistry) RegistryStr() string {
	return o.host
}

func NewOCIRegistry(registryUrl string) (*OCIRegistry, error) {
	fullURL, err := url.Parse(fmt.Sprintf("https://%s", NoSchemeURL(registryUrl)))
	if err != nil {
		return nil, err
	}
	return &OCIRegistry{target: registryUrl, host: fullURL.Host}, nil
}

func NoSchemeURL(url string) string {
	regex := regexp.MustCompile(`^https?://`)
	return regex.ReplaceAllString(url, "")
}

func GetRemoteDescriptor(ctx context.Context,
	descriptor *v1beta1.Descriptor,
	clnt client.Client,
) (*compdesc.ComponentDescriptor, error) {

	repositoryContext := descriptor.GetEffectiveRepositoryContext()
	if repositoryContext == nil {
		return nil, ErrNoEffectiveRepositoryContext
	}
	repoTyped, err := repositoryContext.Evaluate(cpi.DefaultContext().RepositoryTypes())
	if err != nil {
		return nil, fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
	}
	genericSpec := repoTyped.(*genericocireg.RepositorySpec)
	var repo cpi.Repository
	if registryCredValue, found := descriptor.GetLabels().Get(v1beta1.OCIRegistryCredLabel); found {

		labelSelector, err := GenerateLabelSelector(registryCredValue)
		if err != nil {
			return nil, err
		}
		ociRegistry, err := NewOCIRegistry(genericSpec.Name())
		if err != nil {
			return nil, err
		}
		credentials, err := GetCredentials(ctx, labelSelector, ociRegistry, clnt)
		if err != nil {
			return nil, err
		}
		repo, err = cpi.DefaultContext().RepositoryForSpec(genericSpec, credentials)
	} else {
		repo, err = cpi.DefaultContext().RepositoryForSpec(genericSpec)
	}

	if err != nil {
		return nil, fmt.Errorf("error creating repository from spec: %w", err)
	}

	cva, err := repo.LookupComponentVersion(descriptor.GetName(), descriptor.GetVersion())
	if err != nil {
		return nil, err
	}
	return cva.GetDescriptor(), nil
}
