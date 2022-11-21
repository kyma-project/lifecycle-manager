package remote

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// syncContextKey is a singleton key.
var syncContextKey = struct{}{} //nolint:gochecknoglobals

var ErrIsNoSyncContext = errors.New("the given value is not a pointer to a kyma synchronization context")

func InitializeSyncContext(
	ctx context.Context, kyma *v1alpha1.Kyma, controlPlaneClient client.Client, cache *ClientCache,
) (context.Context, error) {
	syncContext, err := InitializeKymaSynchronizationContext(ctx, kyma, controlPlaneClient, cache)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, syncContextKey, syncContext), err
}

func SyncContextFromContext(ctx context.Context) *KymaSynchronizationContext {
	rawCtx := ctx.Value(syncContextKey)
	syncContext, ok := rawCtx.(*KymaSynchronizationContext)
	if !ok {
		panic(ErrIsNoSyncContext)
	}
	return syncContext
}
