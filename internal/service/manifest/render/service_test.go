package render_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

type stubParser struct {
	parseResources internal.ManifestResources
	parseErr       error
	parseCalls     int
	evictedSpecs   []*declarativev2.Spec
}

func (s *stubParser) Parse(*declarativev2.Spec) (internal.ManifestResources, error) {
	s.parseCalls++
	if s.parseErr != nil {
		return internal.ManifestResources{}, s.parseErr
	}
	return s.parseResources, nil
}

func (s *stubParser) EvictCache(spec *declarativev2.Spec) {
	s.evictedSpecs = append(s.evictedSpecs, spec)
}

func newConfigMap(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind((&apimetav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "ConfigMap",
	}).GroupVersionKind())
	obj.SetName(name)
	return obj
}

func TestRenderTargetResources_AppliesTransformsInOrder(t *testing.T) {
	t.Parallel()

	first := newConfigMap("first")
	parser := &stubParser{
		parseResources: internal.ManifestResources{
			Items: []*unstructured.Unstructured{first},
		},
	}

	manifest := &v1beta2.Manifest{}

	var calls []string
	var seenObjs []declarativev2.Object
	transforms := []declarativev2.ResourceTransform{
		func(_ context.Context, obj declarativev2.Object, items []*unstructured.Unstructured) error {
			calls = append(calls, "a")
			seenObjs = append(seenObjs, obj)
			items[0].SetAnnotations(map[string]string{"step": "a"})
			return nil
		},
		func(_ context.Context, obj declarativev2.Object, items []*unstructured.Unstructured) error {
			calls = append(calls, "b")
			seenObjs = append(seenObjs, obj)
			require.Equal(t, "a", items[0].GetAnnotations()["step"])
			items[0].SetAnnotations(map[string]string{"step": "b"})
			return nil
		},
	}

	svc := render.NewService(parser, transforms)

	rendered, err := svc.RenderTargetResources(t.Context(), manifest, &declarativev2.Spec{})

	require.NoError(t, err)
	require.Len(t, rendered, 1)
	assert.Equal(t, []string{"a", "b"}, calls)
	assert.Equal(t, "b", rendered[0].GetAnnotations()["step"])
	assert.Equal(t, 1, parser.parseCalls)
	require.Len(t, seenObjs, 2)
	assert.Same(t, manifest, seenObjs[0])
	assert.Same(t, manifest, seenObjs[1])
}

func TestRenderTargetResources_ReturnsParserError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	svc := render.NewService(&stubParser{parseErr: wantErr}, nil)

	rendered, err := svc.RenderTargetResources(t.Context(), &v1beta2.Manifest{}, &declarativev2.Spec{})

	require.ErrorIs(t, err, wantErr)
	assert.Nil(t, rendered)
}

func TestRenderTargetResources_StopsOnTransformError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("transform failed")
	parser := &stubParser{
		parseResources: internal.ManifestResources{
			Items: []*unstructured.Unstructured{newConfigMap("x")},
		},
	}

	var secondCalled bool
	transforms := []declarativev2.ResourceTransform{
		func(context.Context, declarativev2.Object, []*unstructured.Unstructured) error {
			return wantErr
		},
		func(context.Context, declarativev2.Object, []*unstructured.Unstructured) error {
			secondCalled = true
			return nil
		},
	}

	svc := render.NewService(parser, transforms)

	rendered, err := svc.RenderTargetResources(t.Context(), &v1beta2.Manifest{}, &declarativev2.Spec{})

	require.ErrorIs(t, err, wantErr)
	assert.Nil(t, rendered)
	assert.False(t, secondCalled, "subsequent transforms must not be invoked after an error")
}

func TestEvictCache_DelegatesToParser(t *testing.T) {
	t.Parallel()

	parser := &stubParser{}
	svc := render.NewService(parser, nil)
	spec := &declarativev2.Spec{Path: "some/path", ManifestName: "name"}

	svc.EvictCache(spec)

	require.Len(t, parser.evictedSpecs, 1)
	assert.Same(t, spec, parser.evictedSpecs[0])
}
