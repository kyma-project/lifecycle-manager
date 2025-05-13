package orphan

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const orphanedManifestTolerance = 5 * time.Minute

var ErrOrphanedManifest = errors.New("orphaned manifest detected")

type DetectionRepository interface {
	GetKyma(ctx context.Context, kymaName string, kymaNamespace string) (*v1beta2.Kyma, error)
}

type DetectionService struct {
	repository DetectionRepository
}

func NewDetectionService(repository DetectionRepository) *DetectionService {
	return &DetectionService{
		repository: repository,
	}
}

func (s *DetectionService) DetectOrphanedManifest(ctx context.Context, manifest *v1beta2.Manifest) error {
	if shouldSkipCheck(manifest) {
		return nil
	}

	kyma, err := s.getParentKyma(ctx, manifest)
	if err != nil {
		return fmt.Errorf("error during orphaned manifest detection for manifest %s: %w", manifest.Name, err)
	}

	if !isManifestReferencedInKymaStatus(kyma, manifest.Name) {
		if !isManifestRecentlyCreated(manifest.GetCreationTimestamp().Time) {
			return fmt.Errorf("%w: manifest is not referenced in Kyma status", ErrOrphanedManifest)
		}
	}

	return nil
}

func shouldSkipCheck(manifest *v1beta2.Manifest) bool {
	if manifest.IsMandatoryModule() {
		// Mandatory modules are not refereced by any Kyma object so cannot be orphaned
		return true
	}
	if manifest.GetDeletionTimestamp() != nil {
		// If the manifest is being deleted, we don't check for orphaned status (as it should be eventually deleted)
		return true
	}
	return false
}

func (s *DetectionService) getParentKyma(ctx context.Context, manifest *v1beta2.Manifest) (*v1beta2.Kyma, error) {
	kymaName, err := manifest.GetKymaName()
	if err != nil {
		return nil, fmt.Errorf("cannot get parent Kyma name: %w", err)
	}

	kyma, err := s.repository.GetKyma(ctx, kymaName, manifest.GetNamespace())
	if err != nil {
		if util.IsNotFound(err) {
			return nil, fmt.Errorf("%w: parent Kyma does not exist", ErrOrphanedManifest)
		}
		return nil, fmt.Errorf("cannot fetch parent Kyma object: %w", err)
	}
	return kyma, nil
}

func isManifestReferencedInKymaStatus(kyma *v1beta2.Kyma, targetManifestName string) bool {
	for _, module := range kyma.Status.Modules {
		if module.Manifest != nil && module.Manifest.Name == targetManifestName {
			return true
		}
	}

	return false
}

func isManifestRecentlyCreated(manifestCreationTimestamp time.Time) bool {
	return manifestCreationTimestamp.Add(orphanedManifestTolerance).After(time.Now())
}
