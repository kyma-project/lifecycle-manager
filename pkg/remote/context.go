package remote

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type kymaContextKey = struct{}

var ErrIsNoSyncContext = errors.New("the given value is not a pointer to a kyma synchronization context")

func Set(ctx context.Context, kyma *v1beta2.Kyma
) (context.Context, error) {
	syncContext, err := InitializeKymaSynchronizationContext(ctx, kcp, cache, kyma, syncNamespace)
	if err != nil {
		return nil, fmt.Errorf("initializing sync context failed: %w", err)
	}
	return context.WithValue(ctx, kymaContextKey{}, syncContext), nil
}

func SyncContextFromContext(ctx context.Context) (*KymaSynchronizationContext, error) {
	rawCtx := ctx.Value(kymaContextKey{})
	syncContext, ok := rawCtx.(*KymaSynchronizationContext)
	if !ok {
		return nil, ErrIsNoSyncContext
	}
	return syncContext, nil
}
