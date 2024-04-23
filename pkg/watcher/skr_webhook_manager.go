package watcher

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
)

type SKRWebhookManager interface {
	// Install installs the watcher's webhook chart resources on the SKR cluster
	Install(ctx context.Context, factory remote.SkrContextFactory, kyma *v1beta2.Kyma) error
	// Remove removes the watcher's webhook chart resources from the SKR cluster
	Remove(ctx context.Context, factory remote.SkrContextFactory, kyma *v1beta2.Kyma) error
}
