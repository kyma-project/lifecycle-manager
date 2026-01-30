package usecases

import (
	"context"
)

type SkrAccessSecretRepo interface {
	ExistsForKyma(ctx context.Context, kymaName string) (bool, error)
}

type ExistsDeleteByNameRepo interface {
	Exists(ctx context.Context, name string) (bool, error)
	Delete(ctx context.Context, name string) error
}
