package remote

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// syncContextKey is a singleton key.
type syncContextKey = struct{}

var ErrIsNoSyncContext = errors.New("the given value is not a pointer to a kyma synchronization context")

func InitializeSyncContext(ctx context.Context, kyma *v1beta2.Kyma,
	syncNamespace string, kcp Client, cache *ClientCache,
) (context.Context, error) {
	syncContext, err := InitializeKymaSynchronizationContext(ctx, kcp, cache, kyma, syncNamespace)
	if err != nil {
		return nil, fmt.Errorf("initializing sync context failed: %w", err)
	}
	return context.WithValue(ctx, syncContextKey{}, syncContext), nil
}

func SyncContextFromContext(ctx context.Context) (*KymaSynchronizationContext, error) {
	rawCtx := ctx.Value(syncContextKey{})
	syncContext, ok := rawCtx.(*KymaSynchronizationContext)
	if !ok {
		return nil, ErrIsNoSyncContext
	}
	return syncContext, nil
}
