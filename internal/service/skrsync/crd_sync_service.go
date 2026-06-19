package skrsync

import (
	"context"
	"errors"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var ErrCrdSync = errors.New("failed to sync CRDs to SKR")

// KcpCrdReader reads CustomResourceDefinitions from KCP.
type KcpCrdReader interface {
	Get(ctx context.Context, name string) (*apiextensionsv1.CustomResourceDefinition, error)
}

// SkrCrdApplier applies a single CRD to the SKR cluster of the given Kyma using SSA.
// One applier instance is bound to one CRD name.
type SkrCrdApplier interface {
	Apply(ctx context.Context, kymaName types.NamespacedName, kcpCrd *apiextensionsv1.CustomResourceDefinition) error
}

// CrdSyncService synchronizes the KLM-managed CRDs from KCP to the SKR for a Kyma.
// It resolves the desired CRD definitions from KCP and upserts them on the SKR via SSA so that
// no preceding read on the SKR is required and the API server determines whether action is necessary.
type CrdSyncService struct {
	kcpCrdRepo KcpCrdReader
	appliers   []skrCrdApplier
}

type skrCrdApplier struct {
	kind    shared.Kind
	applier SkrCrdApplier
}

// NewCrdSyncService wires the service with the desired CRD kinds. The order of appliers
// determines the order in which the CRDs are synced. Errors are aggregated and reported as a single error.
func NewCrdSyncService(kcpCrdRepo KcpCrdReader,
	kymaApplier SkrCrdApplier,
	moduleTemplateApplier SkrCrdApplier,
	moduleReleaseMetaApplier SkrCrdApplier,
) *CrdSyncService {
	return &CrdSyncService{
		kcpCrdRepo: kcpCrdRepo,
		appliers: []skrCrdApplier{
			{kind: shared.KymaKind, applier: kymaApplier},
			{kind: shared.ModuleTemplateKind, applier: moduleTemplateApplier},
			{kind: shared.ModuleReleaseMetaKind, applier: moduleReleaseMetaApplier},
		},
	}
}

// Sync upserts the KLM-managed CRDs on the SKR cluster. It always issues SSA patches and
// leaves it to the SKR API server to decide whether a write is necessary.
func (s *CrdSyncService) Sync(ctx context.Context, kyma *v1beta2.Kyma) error {
	kymaName := kyma.GetNamespacedName()

	var errs []error
	for _, entry := range s.appliers {
		if err := s.syncOne(ctx, kymaName, entry); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(append(errs, ErrCrdSync)...)
}

func (s *CrdSyncService) syncOne(ctx context.Context,
	kymaName types.NamespacedName, entry skrCrdApplier,
) error {
	crdName := fmt.Sprintf("%s.%s", entry.kind.Plural(), shared.OperatorGroup)
	kcpCrd, err := s.kcpCrdRepo.Get(ctx, crdName)
	if err != nil {
		return fmt.Errorf("could not read KCP CRD %q: %w", crdName, err)
	}

	if err := entry.applier.Apply(ctx, kymaName, kcpCrd); err != nil {
		return fmt.Errorf("could not apply %s CRD to SKR: %w", entry.kind, err)
	}

	return nil
}
