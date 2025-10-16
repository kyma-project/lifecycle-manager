package usecases

import (
	"context"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ManifestRepo interface {
	ListAllForModule(ctx context.Context, moduleName string) ([]apimetav1.PartialObjectMetadata, error)
	DeleteAllForModule(ctx context.Context, moduleName string) error
}

// DeleteManifests is responsible for deleting all manifests associated with a ModuleReleaseMeta.
type DeleteManifests struct {
	repo ManifestRepo
}

func NewDeleteManifests(repo ManifestRepo) *DeleteManifests {
	return &DeleteManifests{repo: repo}
}

// IsApplicable returns true if the ModuleReleaseMeta has associated manifests, so they should be deleted.
func (d *DeleteManifests) IsApplicable(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) (bool, error) {
	manifests, err := d.repo.ListAllForModule(ctx, mrm.Name)
	if err != nil {
		return false, fmt.Errorf("failed to list manifests for module %s: %w", mrm.Name, err)
	}
	return len(manifests) > 0, nil
}

func (d *DeleteManifests) Execute(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error {
	if err := d.repo.DeleteAllForModule(ctx, mrm.Name); err != nil {
		return fmt.Errorf("failed to delete manifests for module %s: %w", mrm.Name, err)
	}
	return nil
}
