package modulereleasemeta

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type kymaRepository interface {
	LookupByLabel(ctx context.Context, labelKey, labelValue string) (*v1beta2.KymaList, error)
}
