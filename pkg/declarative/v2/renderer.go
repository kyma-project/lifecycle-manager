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
