package v2

import (
	"context"
)

type SpecResolver interface {
	Spec(ctx context.Context, object Object) (*Spec, error)
}

type Spec struct {
	ManifestName string
	Path         string
	Values       any
	Mode         RenderMode
}

func DefaultSpec(path string, values any, mode RenderMode) *CustomSpecFns {
	return &CustomSpecFns{
		ManifestNameFn: func(_ context.Context, obj Object) string { return obj.ComponentName() },
		PathFn:         func(_ context.Context, _ Object) string { return path },
		ValuesFn:       func(_ context.Context, _ Object) any { return values },
		ModeFn:         func(_ context.Context, _ Object) RenderMode { return mode },
	}
}

// CustomSpecFns is a simple static resolver that always uses the same chart and values.
type CustomSpecFns struct {
	ManifestNameFn func(ctx context.Context, obj Object) string
	PathFn         func(ctx context.Context, obj Object) string
	ValuesFn       func(ctx context.Context, obj Object) any
	ModeFn         func(ctx context.Context, obj Object) RenderMode
}

func (s *CustomSpecFns) Spec(
	ctx context.Context, obj Object,
) (*Spec, error) {
	return &Spec{
		ManifestName: s.ManifestNameFn(ctx, obj),
		Path:         s.PathFn(ctx, obj),
		Values:       s.ValuesFn(ctx, obj),
		Mode:         s.ModeFn(ctx, obj),
	}, nil
}

type RenderMode string

const (
	RenderModeHelm      RenderMode = "helm"
	RenderModeKustomize RenderMode = "kustomize"
	RenderModeRaw       RenderMode = "raw"
)
