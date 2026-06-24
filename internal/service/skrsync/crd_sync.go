package skrsync

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// ErrCrdSync is the sentinel error wrapping all failures that occurred during a CRD sync run.
var ErrCrdSync = errors.New("failed to sync CRDs to SKR")

// SkrCrdSyncEntry pairs a KLM-managed CRD kind with the applier responsible for syncing it to the SKR.
// The kind is used both to resolve the KCP CRD name and to label errors.
type SkrCrdSyncEntry struct {
	Kind    shared.Kind
	Applier SkrCrdApplier
}

// crdSync executes the CRD synchronization for a single Kyma. It reads the desired CRD definitions
// from KCP and applies them on the SKR via Server-Side Apply. Errors across entries are aggregated;
// one failure does not stop subsequent entries so that as many CRDs as possible reach the SKR.
type crdSync struct {
	kcpCrdReader KcpCrdReader
	entries      []SkrCrdSyncEntry
}

func newCrdSync(kcpCrdReader KcpCrdReader, entries []SkrCrdSyncEntry) *crdSync {
	return &crdSync{
		kcpCrdReader: kcpCrdReader,
		entries:      entries,
	}
}

func (c *crdSync) execute(ctx context.Context, kyma *v1beta2.Kyma) error {
	kymaName := kyma.GetNamespacedName()

	var errs []error
	for _, entry := range c.entries {
		if err := c.applyOne(ctx, kymaName, entry); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.Join(append([]error{ErrCrdSync}, errs...)...)
}

func (c *crdSync) applyOne(ctx context.Context, kymaName types.NamespacedName, entry SkrCrdSyncEntry) error {
	name := qualifiedCrdName(entry.Kind)

	kcpCrd, err := c.kcpCrdReader.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("could not read KCP CRD %q: %w", name, err)
	}

	if err := entry.Applier.Apply(ctx, kymaName, kcpCrd); err != nil {
		return fmt.Errorf("could not apply %s CRD to SKR: %w", entry.Kind, err)
	}

	return nil
}

func qualifiedCrdName(kind shared.Kind) string {
	return fmt.Sprintf("%s.%s", kind.Plural(), shared.OperatorGroup)
}
