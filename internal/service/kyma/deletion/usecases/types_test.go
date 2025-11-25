package usecases_test

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
)

type skrAccessSecretRepoStub struct {
	usecases.SkrAccessSecretRepo

	called   bool
	kymaName string
	exists   bool
	err      error
}

func (r *skrAccessSecretRepoStub) Exists(_ context.Context, kymaName string) (bool, error) {
	r.called = true
	r.kymaName = kymaName
	return r.exists, r.err
}
