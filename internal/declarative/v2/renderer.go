package v2

import (
	"context"
	"errors"

	"k8s.io/cli-runtime/pkg/resource"
)

var (
	ErrConditionsNotYetRegistered = errors.New("conditions have not yet been registered in status")
	ErrPrerequisitesNotFulfilled  = errors.New("prerequisites for installation are not fulfilled")
)

type Prerequisites []*resource.Info

type ConditionsNeedUpdate bool

type Renderer interface {
	Initialize(obj Object) error
	EnsurePrerequisites(ctx context.Context, obj Object) error
	Render(ctx context.Context, obj Object) ([]byte, error)
	RemovePrerequisites(ctx context.Context, obj Object) error
}

type RenderMode string

const (
	RenderModeHelm      RenderMode = "helm"
	RenderModeKustomize RenderMode = "kustomize"
	RenderModeRaw       RenderMode = "raw"
)

func InitializeRenderer(
	ctx context.Context,
	obj Object,
	spec *Spec,
	client Client,
	options *Options,
) (Renderer, error) {
	var renderer Renderer

	switch spec.Mode {
	case RenderModeHelm:
		renderer = NewHelmRenderer(spec, client, options)
		renderer = WrapWithRendererCache(renderer, spec, options)
	case RenderModeKustomize:
		renderer = NewKustomizeRenderer(spec, options)
		renderer = WrapWithRendererCache(renderer, spec, options)
	case RenderModeRaw:
		renderer = NewRawRenderer(spec,client, options)
	}

	if err := renderer.Initialize(obj); err != nil {
		return nil, err
	}
	if err := renderer.EnsurePrerequisites(ctx, obj); err != nil {
		return nil, err
	}

	return renderer, nil
}
