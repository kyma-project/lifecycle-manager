package usecases

import (
	"context"
)

type SkrAccessSecretRepo interface {
	ExistsForKyma(ctx context.Context, kymaName string) (bool, error)
}
