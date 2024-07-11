package v2

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type SpecResolver interface {
	GetSpec(ctx context.Context, manifest *v1beta2.Manifest) (*Spec, error)
}

type Spec struct {
	ManifestName string
	Path         string
	OCIRef       string
}
