package context

import (
	"context"
	"errors"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type kymaContextKey = struct{}

var ErrKymaCtxNotFound = errors.New("context contains no value for kyma context")

func Set(ctx context.Context, kyma *v1beta2.Kyma) context.Context {
	return context.WithValue(ctx, kymaContextKey{}, kyma.Name)
}

func Get(ctx context.Context) (*string, error) {
	rawCtx := ctx.Value(kymaContextKey{})
	syncContext, ok := rawCtx.(*string)
	if !ok {
		return nil, ErrKymaCtxNotFound
	}
	return syncContext, nil
}
