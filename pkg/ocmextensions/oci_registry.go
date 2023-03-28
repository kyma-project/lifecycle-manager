package ocmextensions

import (
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/cpi"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/repositories/genericocireg"
)

var (
	ErrNoEffectiveRepositoryContext = errors.New("no effective repository context")
)

func GetRemoteDescriptor(descriptor *v1beta1.Descriptor) (*compdesc.ComponentDescriptor, error) {

	ctx := descriptor.GetEffectiveRepositoryContext()
	if ctx == nil {
		return nil, ErrNoEffectiveRepositoryContext
	}
	repoTyped, err := ctx.Evaluate(cpi.DefaultContext().RepositoryTypes())
	if err != nil {
		return nil, fmt.Errorf("error while decoding the repository context into an OCI registry: %w", err)
	}
	genericSpec := repoTyped.(*genericocireg.RepositorySpec)
	//TODO: add cred support
	repo, err := cpi.DefaultContext().RepositoryForSpec(genericSpec)

	if err != nil {
		return nil, fmt.Errorf("error creating repository from spec: %w", err)
	}

	cva, err := repo.LookupComponentVersion(descriptor.GetName(), descriptor.GetVersion())
	if err != nil {
		return nil, err
	}
	return cva.GetDescriptor(), nil
}
