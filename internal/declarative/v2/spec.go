package v2

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SpecResolver interface {
	Spec(ctx context.Context, object Object, remoteClient client.Client) (*Spec, error)
}

type Spec struct {
	ManifestName string
	Path         string
	OCIRef       string
	Mode         RenderMode
}

func DefaultSpec(path, ociref string, mode RenderMode) *CustomSpecFns {
	return &CustomSpecFns{
		ManifestNameFn: func(_ context.Context, obj Object) string { return obj.GetName() },
		PathFn:         func(_ context.Context, _ Object) string { return path },
		OCIRefFn:       func(_ context.Context, _ Object) string { return ociref },
		ModeFn:         func(_ context.Context, _ Object) RenderMode { return mode },
	}
}

// CustomSpecFns is a simple static resolver that always uses the same chart and values.
type CustomSpecFns struct {
	ManifestNameFn func(ctx context.Context, obj Object) string
	PathFn         func(ctx context.Context, obj Object) string
	OCIRefFn       func(ctx context.Context, obj Object) string
	ModeFn         func(ctx context.Context, obj Object) RenderMode
}

func (s *CustomSpecFns) Spec(
	ctx context.Context, obj Object, _ client.Client,
) (*Spec, error) {
	return &Spec{
		ManifestName: s.ManifestNameFn(ctx, obj),
		Path:         s.PathFn(ctx, obj),
		OCIRef:       s.OCIRefFn(ctx, obj),
		Mode:         s.ModeFn(ctx, obj),
	}, nil
}
