package usecases_test

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/deletion/usecases"
	"k8s.io/apimachinery/pkg/types"
)

type kymaStatusRepoStub struct {
	usecases.KymaStatusRepo

	called         bool
	namespacedName types.NamespacedName
	status         *v1beta2.KymaStatus
	err            error
}

func (r *kymaStatusRepoStub) Get(_ context.Context, namespacedName types.NamespacedName) (*v1beta2.KymaStatus, error) {
	r.called = true
	r.namespacedName = namespacedName
	return r.status, r.err
}

func (r *kymaStatusRepoStub) SetStateDeleting(_ context.Context, namespacedName types.NamespacedName) error {
	r.called = true
	r.namespacedName = namespacedName
	return r.err
}
