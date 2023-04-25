package watcher

import (
	"context"
)

type SKRWebhookManager interface {
	// Install installs the watcher's webhook chart resources on the SKR cluster
	Install(ctx context.Context, kyma *v1beta2.Kyma) error
	// Remove removes the watcher's webhook chart resources from the SKR cluster
	Remove(ctx context.Context, kyma *v1beta2.Kyma) error
}
