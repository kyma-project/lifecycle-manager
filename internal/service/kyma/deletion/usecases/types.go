package usecases

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

type SkrAccessSecretRepo interface {
	ExistsForKyma(ctx context.Context, kymaName string) (bool, error)
}

type ExistsDeleteByKymaNameRepo interface {
	Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error)
	Delete(ctx context.Context, kymaName types.NamespacedName) error
}

type ExistsDeleteByNameRepo interface {
	Exists(ctx context.Context, name string) (bool, error)
	Delete(ctx context.Context, name string) error
}
