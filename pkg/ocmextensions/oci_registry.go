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
	"github.com/open-component-model/ocm/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoEffectiveRepositoryContext = errors.New("no effective repository context")

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

func NewOCIRegistry(registryURL string) (*OCIRegistry, error) {
	fullURL, err := url.Parse(fmt.Sprintf("https://%s", NoSchemeURL(registryURL)))
	if err != nil {
		return nil, err
	}
	return &OCIRegistry{target: registryURL, host: fullURL.Host}, nil
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
	repo, err := GetRepo(ctx, descriptor, clnt, repoTyped)
	if err != nil {
		return nil, fmt.Errorf("error creating repository from spec: %w", err)
	}
	cva, err := repo.LookupComponentVersion(descriptor.GetName(), descriptor.GetVersion())
	if err != nil {
		return nil, err
	}
	return cva.GetDescriptor(), nil
}

func GetRepo(ctx context.Context,
	descriptor *v1beta1.Descriptor,
	clnt client.Client,
	repoTyped runtime.TypedObject,
) (cpi.Repository, error) {
	genericSpec := repoTyped.(*genericocireg.RepositorySpec)
	if registryCredValue, found := descriptor.GetLabels().Get(v1beta1.OCIRegistryCredLabel); found {
		ociRegistry, err := NewOCIRegistry(genericSpec.Name())
		if err != nil {
			return nil, err
		}
		cred, err := GetCredentials(ctx, registryCredValue, ociRegistry, clnt)
		if err != nil {
			return nil, err
		}
		return cpi.DefaultContext().RepositoryForSpec(genericSpec, cred)
	}
	return cpi.DefaultContext().RepositoryForSpec(genericSpec)
}
