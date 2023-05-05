package sync

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GetModuleFunc func(ctx context.Context, module client.Object) error
