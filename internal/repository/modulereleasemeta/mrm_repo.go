package modulereleasemeta

import (
	"context"
	"fmt"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type Repository struct {
	clnt      client.Client
	namespace string
}

func NewRepository(clnt client.Client, namespace string) *Repository {
	return &Repository{
		clnt:      clnt,
		namespace: namespace,
	}
}

func (r *Repository) EnsureFinalizer(ctx context.Context, mrmName string, finalizer string) error {
	mrm, err := r.Get(ctx, mrmName)
	if err != nil {
		return err
	}
	if !slices.Contains(mrm.Finalizers, finalizer) {
		mrm.Finalizers = append(mrm.Finalizers, finalizer)
		if err := r.clnt.Update(ctx, mrm); err != nil {
			return fmt.Errorf("failed to add finalizer to ModuleReleaseMeta %s: %w", mrmName, err)
		}
	}
	return nil
}

func (r *Repository) RemoveFinalizer(ctx context.Context, mrmName string, finalizer string) error {
	mrm, err := r.Get(ctx, mrmName)
	if err != nil {
		return err
	}
	mrm.Finalizers = slices.DeleteFunc(mrm.Finalizers, func(s string) bool { return s == finalizer })
	if err := r.clnt.Update(ctx, mrm); err != nil {
		return fmt.Errorf("failed to remove finalizer from ModuleReleaseMeta %s: %w", mrmName, err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, mrmName string) (*v1beta2.ModuleReleaseMeta, error) {
	mrm := &v1beta2.ModuleReleaseMeta{}
	err := r.clnt.Get(ctx, client.ObjectKey{Name: mrmName, Namespace: r.namespace}, mrm)
	if err != nil {
		return nil, fmt.Errorf("failed to get ModuleReleaseMeta %s in namespace %s: %w", mrmName, r.namespace, err)
	}
	return mrm, nil
}
