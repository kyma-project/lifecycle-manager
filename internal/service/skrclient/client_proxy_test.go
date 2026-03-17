package skrclient

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgocorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// fakeResettableMapper is a RESTMapper that returns NoKindMatchError on the first
// RESTMapping call, then succeeds on all subsequent calls. It also implements
// ResettableRESTMapper so that MaybeResetRESTMapper triggers the reset.
type fakeResettableMapper struct {
	callCount  int
	resetCount int
	mapping    *apimeta.RESTMapping
}

func (f *fakeResettableMapper) Reset() {
	f.resetCount++
}

func (f *fakeResettableMapper) KindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeResettableMapper) KindsFor(_ schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}
func (f *fakeResettableMapper) ResourceFor(_ schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}
func (f *fakeResettableMapper) ResourcesFor(_ schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, nil
}
func (f *fakeResettableMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*apimeta.RESTMapping, error) {
	f.callCount++
	if f.callCount == 1 {
		return nil, &apimeta.NoKindMatchError{GroupKind: gk}
	}
	return f.mapping, nil
}
func (f *fakeResettableMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*apimeta.RESTMapping, error) {
	return nil, nil
}
func (f *fakeResettableMapper) AbbreviatedKindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeResettableMapper) ResourceSingularizer(_ string) (string, error) {
	return "", nil
}

// fakeAlwaysNoMatchMapper always returns NoKindMatchError.
type fakeAlwaysNoMatchMapper struct{}

func (f *fakeAlwaysNoMatchMapper) KindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeAlwaysNoMatchMapper) KindsFor(_ schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}
func (f *fakeAlwaysNoMatchMapper) ResourceFor(_ schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}
func (f *fakeAlwaysNoMatchMapper) ResourcesFor(_ schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, nil
}
func (f *fakeAlwaysNoMatchMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*apimeta.RESTMapping, error) {
	return nil, &apimeta.NoKindMatchError{GroupKind: gk}
}
func (f *fakeAlwaysNoMatchMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*apimeta.RESTMapping, error) {
	return nil, nil
}
func (f *fakeAlwaysNoMatchMapper) AbbreviatedKindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeAlwaysNoMatchMapper) ResourceSingularizer(_ string) (string, error) {
	return "", nil
}

// fakeSuccessMapper always succeeds.
type fakeSuccessMapper struct {
	callCount int
	mapping   *apimeta.RESTMapping
}

func (f *fakeSuccessMapper) KindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeSuccessMapper) KindsFor(_ schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}
func (f *fakeSuccessMapper) ResourceFor(_ schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, nil
}
func (f *fakeSuccessMapper) ResourcesFor(_ schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, nil
}
func (f *fakeSuccessMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*apimeta.RESTMapping, error) {
	f.callCount++
	return f.mapping, nil
}
func (f *fakeSuccessMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*apimeta.RESTMapping, error) {
	return nil, nil
}
func (f *fakeSuccessMapper) AbbreviatedKindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}
func (f *fakeSuccessMapper) ResourceSingularizer(_ string) (string, error) {
	return "", nil
}

func podMapping() *apimeta.RESTMapping {
	return &apimeta.RESTMapping{
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Scope:            apimeta.RESTScopeNamespace,
	}
}

// newProxyWithFakeBase builds a ProxyClient with a given mapper and an interceptable fake base client.
func newProxyWithFakeBase(mapper apimeta.RESTMapper, applyFn func(context.Context, client.WithWatch, machineryruntime.ApplyConfiguration, ...client.ApplyOption) error) *ProxyClient {
	base := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Apply: applyFn,
	}).Build()
	return &ProxyClient{
		config:     &rest.Config{},
		mapper:     mapper,
		baseClient: base,
	}
}

// TestProxyClientApply_TypedApplyConfiguration verifies that a typed apply config
// (PodApplyConfiguration with APIVersion+Kind) is delegated to the baseClient.
func TestProxyClientApply_TypedApplyConfiguration(t *testing.T) {
	applied := false
	mapper := &fakeSuccessMapper{mapping: podMapping()}
	proxy := newProxyWithFakeBase(mapper, func(_ context.Context, _ client.WithWatch, _ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption) error {
		applied = true
		return nil
	})

	podAC := clientgocorev1.Pod("test-pod", "default")
	err := proxy.Apply(t.Context(), podAC)

	require.NoError(t, err)
	assert.True(t, applied, "baseClient.Apply should have been called")
	assert.Equal(t, 1, mapper.callCount, "mapper should have been called once")
}

// TestProxyClientApply_UnstructuredApplyConfiguration verifies that ApplyConfigurationFromUnstructured
// wrapping a Deployment is delegated and the mapper is called with the correct GVK.
func TestProxyClientApply_UnstructuredApplyConfiguration(t *testing.T) {
	deployMapping := &apimeta.RESTMapping{
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Scope:            apimeta.RESTScopeNamespace,
	}
	mapper := &fakeSuccessMapper{mapping: deployMapping}
	applied := false
	proxy := newProxyWithFakeBase(mapper, func(_ context.Context, _ client.WithWatch, _ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption) error {
		applied = true
		return nil
	})

	u := &unstructured.Unstructured{}
	u.SetAPIVersion("apps/v1")
	u.SetKind("Deployment")
	u.SetName("test-deployment")
	u.SetNamespace("default")
	ac := client.ApplyConfigurationFromUnstructured(u)

	err := proxy.Apply(t.Context(), ac)

	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 1, mapper.callCount)
}

// TestProxyClientApply_NoMatchErrorThenReset verifies that a NoKindMatchError on the first
// mapper call triggers a reset (via MaybeResetRESTMapper) and a second call that succeeds.
func TestProxyClientApply_NoMatchErrorThenReset(t *testing.T) {
	mapper := &fakeResettableMapper{mapping: podMapping()}
	applied := false
	proxy := newProxyWithFakeBase(mapper, func(_ context.Context, _ client.WithWatch, _ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption) error {
		applied = true
		return nil
	})

	podAC := clientgocorev1.Pod("test-pod", "default")
	err := proxy.Apply(t.Context(), podAC)

	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 1, mapper.resetCount, "mapper Reset should have been called once")
	assert.Equal(t, 2, mapper.callCount, "mapper should have been called twice (first fail, then after reset)")
}

// TestProxyClientApply_PersistentNoMatchError verifies that when the mapper always returns
// NoKindMatchError (even after reset), Apply returns an error.
func TestProxyClientApply_PersistentNoMatchError(t *testing.T) {
	proxy := newProxyWithFakeBase(&fakeAlwaysNoMatchMapper{}, nil)

	podAC := clientgocorev1.Pod("test-pod", "default")
	err := proxy.Apply(t.Context(), podAC)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get resource mapping")
}

// TestProxyClientApply_MissingAPIVersion verifies that an apply configuration without
// an apiVersion returns ErrMissingAPIVersion.
func TestProxyClientApply_MissingAPIVersion(t *testing.T) {
	// PodApplyConfiguration without calling WithAPIVersion — pointer remains nil.
	ac := &clientgocorev1.PodApplyConfiguration{}
	ac.WithKind("Pod")

	_, err := gvkFromApplyConfiguration(ac)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingAPIVersion)
}

// TestProxyClientApply_MissingKind verifies that an apply configuration without
// a kind returns ErrMissingKind.
func TestProxyClientApply_MissingKind(t *testing.T) {
	ac := &clientgocorev1.PodApplyConfiguration{}
	ac.WithAPIVersion("v1")
	// Kind is intentionally not set.

	_, err := gvkFromApplyConfiguration(ac)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingKind)
}

// opaqueApplyConfig implements runtime.ApplyConfiguration (the marker interface) but does NOT
// satisfy applyConfigurationGVKAccessor, forcing the JSON fallback path.
type opaqueApplyConfig struct {
	marshalErr bool
}

func (*opaqueApplyConfig) IsApplyConfiguration() {}

func (o *opaqueApplyConfig) MarshalJSON() ([]byte, error) {
	if o.marshalErr {
		return nil, errors.New("forced marshal error")
	}
	return []byte(`{}`), nil
}

// TestProxyClientApply_JSONFallbackMarshalError verifies that when the opaque fallback path
// encounters a JSON marshal error, the error is surfaced correctly.
func TestProxyClientApply_JSONFallbackMarshalError(t *testing.T) {
	ac := &opaqueApplyConfig{marshalErr: true}

	_, err := gvkFromApplyConfiguration(ac)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal apply configuration")
}
