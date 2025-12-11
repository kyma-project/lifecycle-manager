package usecases

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type SkrAccessSecretRepo interface {
	Exists(ctx context.Context, name string) (bool, error)
}

type ExistsDeleteRepo interface {
	Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	Delete(ctx context.Context, kymaName types.NamespacedName) error
}
