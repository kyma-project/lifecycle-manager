package modulereleasemeta

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type kymaRepository interface {
	LookupByLabel(ctx context.Context, labelKey, labelValue string) (*v1beta2.KymaList, error)
}

// affectedKymasResolver decides which Kymas must be requeued for a given ModuleReleaseMeta event.
// Regular and mandatory modules differ only in this decision; the requeue and delay mechanics are shared.
type affectedKymasResolver interface {
	OnCreate(mrm *v1beta2.ModuleReleaseMeta, kymas *v1beta2.KymaList) []*types.NamespacedName
	OnUpdate(oldMRM, newMRM *v1beta2.ModuleReleaseMeta, kymas *v1beta2.KymaList) []*types.NamespacedName
	OnDelete(mrm *v1beta2.ModuleReleaseMeta, kymas *v1beta2.KymaList) []*types.NamespacedName
}
