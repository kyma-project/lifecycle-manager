package remote

import (
	"context"
	kymactx "github.com/kyma-project/lifecycle-manager/internal/controller/kyma/context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EnsureSkrConnectivityUC interface {
	Execute(ctx context.Context) error
}

type EnsureSkrConnectivityImpl struct {
}

func (u *EnsureSkrConnectivityImpl) Execute(ctx context.Context) error {
	kyma, err := kymactx.Get(ctx)
	if err != nil {
		return err
	}
	skrClient, err := u.clientLookup.Lookup(ctx, client.ObjectKeyFromObject(kyma))
	if err != nil {
		return nil, err
	}

	syncContext := &KymaClient{
		Client: skrClient,
	}

	if err := syncContext.ensureNamespaceExists(ctx); err != nil {
		return nil, err
	}

	return syncContext, nil
	return nil
}
