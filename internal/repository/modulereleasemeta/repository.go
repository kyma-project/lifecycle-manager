package modulereleasemeta

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Repository struct {
	client.Reader
}

func NewRepository(reader client.Reader) *Repository {
	return &Repository{
		Reader: reader,
	}
}

func (c *Repository) GetModuleReleaseMeta(ctx context.Context, mrmName string, mrmNamespace string) (
	*v1beta2.ModuleReleaseMeta, error,
) {
	mrm := &v1beta2.ModuleReleaseMeta{}
	err := c.Get(ctx, client.ObjectKey{Name: mrmName, Namespace: mrmNamespace}, mrm)
	if err != nil {
		return nil, fmt.Errorf("failed to get ModuleReleaseMeta %s in namespace %s: %w",
			mrmName, mrmNamespace, err)
	}
	return mrm, nil
}
