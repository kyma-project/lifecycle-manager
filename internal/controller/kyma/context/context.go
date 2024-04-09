package context

import (
	"context"
	"errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type kymaContextKey = struct{}

var (
	ErrKymaCtxNotFound = errors.New("context contains no value for kyma context")
	ErrKymaNameEmpty   = errors.New("kyma name is empty")
)

func Set(ctx context.Context, kyma *v1beta2.Kyma) (context.Context, error) {
	value := types.NamespacedName{Name: kyma.Name, Namespace: kyma.Namespace}
	if value.Name == "" || value.Namespace == "" {
		return nil, ErrKymaNameEmpty
	}
	return context.WithValue(ctx, kymaContextKey{}, value), nil
}

func Get(ctx context.Context) (types.NamespacedName, error) {
	rawValue := ctx.Value(kymaContextKey{})
	if rawValue == nil {
		return types.NamespacedName{}, ErrKymaCtxNotFound
	}
	value, ok := rawValue.(types.NamespacedName)
	if !ok {
		return types.NamespacedName{}, ErrKymaCtxNotFound
	}
	return value, nil
}
