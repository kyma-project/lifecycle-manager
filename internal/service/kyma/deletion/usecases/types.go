package usecases

import "context"

type SkrAccessSecretRepo interface {
	Exists(ctx context.Context, name string) (bool, error)
}
