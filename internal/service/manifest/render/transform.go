package render

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// ResourceTransform mutates the parsed manifest resources in place. Transforms
// run in the order they are passed to Service. A transform may inspect the
// owning Manifest CR for context (e.g. to resolve module-specific fields).
type ResourceTransform = func(ctx context.Context, manifest *v1beta2.Manifest,
	resources []*unstructured.Unstructured) error
