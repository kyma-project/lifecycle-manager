package render

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

// ResourceTransform mutates the parsed manifest resources in place. Transforms
// run in the order they are passed to Service. A transform may inspect the
// owning Manifest CR for context (e.g. to resolve module-specific fields).
type ResourceTransform = func(ctx context.Context, manifest *v1beta2.Manifest,
	resources []*unstructured.Unstructured) error

// CachedManifestParser is the consumer-defined interface for the manifest
// parser used by Service. Service owns the parser to keep the parse cache
// scoped to the render lifecycle.
type CachedManifestParser interface {
	Parse(spec *spec.Spec) (internal.ManifestResources, error)
	EvictCache(spec *spec.Spec)
}
