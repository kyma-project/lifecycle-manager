package deploy

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
)

type SKRWebhookManager interface {
	// Install installs the watcher's webhook chart resources on the SKR cluster
	Install(ctx context.Context, kyma *v1alpha1.Kyma) error
	// Remove removes the watcher's webhook chart resources from the SKR cluster
	Remove(ctx context.Context, kyma *v1alpha1.Kyma) error
}
