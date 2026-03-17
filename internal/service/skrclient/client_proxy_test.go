package skrclient_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

// fakeResettableMapper returns NoKindMatchError on the first RESTMapping call then succeeds.
// It implements ResettableRESTMapper so MaybeResetRESTMapper triggers Reset.
type fakeResettableMapper struct {
	callCount  int
	resetCount int
	mapping    *meta.RESTMapping
}

func (f *fakeResettableMapper) Reset() { f.resetCount++ }

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

func (f *fakeResettableMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	f.callCount++
	if f.callCount == 1 {
		return nil, &meta.NoKindMatchError{GroupKind: gk}
	}
	return f.mapping, nil
}

func (f *fakeResettableMapper) RESTMappings(_ schema.GroupKind, _ ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}

func (f *fakeResettableMapper) AbbreviatedKindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (f *fakeResettableMapper) ResourceSingularizer(_ string) (string, error) { return "", nil }

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

func (f *fakeAlwaysNoMatchMapper) RESTMapping(gk schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	return nil, &meta.NoKindMatchError{GroupKind: gk}
}

func (f *fakeAlwaysNoMatchMapper) RESTMappings(_ schema.GroupKind, _ ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}

func (f *fakeAlwaysNoMatchMapper) AbbreviatedKindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (f *fakeAlwaysNoMatchMapper) ResourceSingularizer(_ string) (string, error) { return "", nil }

// fakeSuccessMapper always succeeds.
type fakeSuccessMapper struct {
	callCount int
	mapping   *meta.RESTMapping
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

func (f *fakeSuccessMapper) RESTMapping(_ schema.GroupKind, _ ...string) (*meta.RESTMapping, error) {
	f.callCount++
	return f.mapping, nil
}

func (f *fakeSuccessMapper) RESTMappings(_ schema.GroupKind, _ ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}

func (f *fakeSuccessMapper) AbbreviatedKindFor(_ schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (f *fakeSuccessMapper) ResourceSingularizer(_ string) (string, error) { return "", nil }

func podMapping() *meta.RESTMapping {
	return &meta.RESTMapping{
		Resource:         schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		GroupVersionKind: schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Scope:            meta.RESTScopeNamespace,
	}
}

// newTestSKRClient builds an SKRClient and returns the underlying *ProxyClient for injection.
func newTestSKRClient(t *testing.T) (*skrclient.SKRClient, *skrclient.ProxyClient) {
	t.Helper()
	manifest := &v1beta2.Manifest{}
	manifest.SetLabels(map[string]string{shared.KymaName: "kyma-test"})
	manifest.SetName("test-manifest")
	manifest.SetNamespace("default")

	svc := skrclient.NewService(1, 1, &FakeAccessManagerService{})
	skrClient, err := svc.ResolveClient(t.Context(), manifest)
	require.NoError(t, err)

	proxy, ok := skrClient.Client.(*skrclient.ProxyClient)
	require.True(t, ok, "embedded client must be *ProxyClient")

	return skrClient, proxy
}

// TestProxyClientApply_TypedApplyConfiguration verifies that a typed apply config
// (PodApplyConfiguration with APIVersion+Kind set) is delegated to the baseClient.
func TestProxyClientApply_TypedApplyConfiguration(t *testing.T) {
	applied := false
	mapper := &fakeSuccessMapper{mapping: podMapping()}
	skrClient, proxy := newTestSKRClient(t)
	proxy.SetMapper(mapper)
	proxy.SetBaseClient(fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Apply: func(_ context.Context, _ client.WithWatch,
			_ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption,
		) error {
			applied = true
			return nil
		},
	}).Build())

	err := skrClient.Apply(t.Context(), corev1ac.Pod("test-pod", "default"))

	require.NoError(t, err)
	assert.True(t, applied, "baseClient.Apply should have been called")
	assert.Equal(t, 1, mapper.callCount, "mapper should have been called once")
}

// TestProxyClientApply_UnstructuredApplyConfiguration verifies that
// ApplyConfigurationFromUnstructured wrapping a Deployment is delegated correctly.
func TestProxyClientApply_UnstructuredApplyConfiguration(t *testing.T) {
	deployMapping := &meta.RESTMapping{
		Resource:         schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		GroupVersionKind: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		Scope:            meta.RESTScopeNamespace,
	}
	applied := false
	mapper := &fakeSuccessMapper{mapping: deployMapping}
	skrClient, proxy := newTestSKRClient(t)
	proxy.SetMapper(mapper)
	proxy.SetBaseClient(fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Apply: func(_ context.Context, _ client.WithWatch,
			_ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption,
		) error {
			applied = true
			return nil
		},
	}).Build())

	uObj := &unstructured.Unstructured{}
	uObj.SetAPIVersion("apps/v1")
	uObj.SetKind("Deployment")
	uObj.SetName("test-deployment")
	uObj.SetNamespace("default")

	err := skrClient.Apply(t.Context(), client.ApplyConfigurationFromUnstructured(uObj))

	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 1, mapper.callCount)
}

// TestProxyClientApply_NoMatchErrorThenReset verifies that a NoKindMatchError on the first
// mapper call triggers a reset and a second successful call.
func TestProxyClientApply_NoMatchErrorThenReset(t *testing.T) {
	applied := false
	mapper := &fakeResettableMapper{mapping: podMapping()}
	skrClient, proxy := newTestSKRClient(t)
	proxy.SetMapper(mapper)
	proxy.SetBaseClient(fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Apply: func(_ context.Context, _ client.WithWatch,
			_ machineryruntime.ApplyConfiguration, _ ...client.ApplyOption,
		) error {
			applied = true
			return nil
		},
	}).Build())

	err := skrClient.Apply(t.Context(), corev1ac.Pod("test-pod", "default"))

	require.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, 1, mapper.resetCount, "mapper Reset should have been called once")
	assert.Equal(t, 2, mapper.callCount, "mapper should have been called twice (first fail, then after reset)")
}

// TestProxyClientApply_PersistentNoMatchError verifies that when the mapper always returns
// NoKindMatchError, Apply returns an error.
func TestProxyClientApply_PersistentNoMatchError(t *testing.T) {
	skrClient, proxy := newTestSKRClient(t)
	proxy.SetMapper(&fakeAlwaysNoMatchMapper{})

	err := skrClient.Apply(t.Context(), corev1ac.Pod("test-pod", "default"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get resource mapping")
}

// TestProxyClientApply_MissingAPIVersion verifies that an apply configuration without
// an apiVersion returns ErrMissingAPIVersion.
func TestProxyClientApply_MissingAPIVersion(t *testing.T) {
	skrClient, _ := newTestSKRClient(t)

	ac := &corev1ac.PodApplyConfiguration{}
	ac.WithKind("Pod")

	err := skrClient.Apply(t.Context(), ac)

	require.Error(t, err)
	assert.ErrorIs(t, err, skrclient.ErrMissingAPIVersion)
}

// TestProxyClientApply_MissingKind verifies that an apply configuration without
// a kind returns ErrMissingKind.
func TestProxyClientApply_MissingKind(t *testing.T) {
	skrClient, _ := newTestSKRClient(t)

	ac := &corev1ac.PodApplyConfiguration{}
	ac.WithAPIVersion("v1")

	err := skrClient.Apply(t.Context(), ac)

	require.Error(t, err)
	assert.ErrorIs(t, err, skrclient.ErrMissingKind)
}

// opaqueApplyConfig implements runtime.ApplyConfiguration but does NOT satisfy
// applyConfigurationGVKAccessor, forcing the JSON fallback path.
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

// TestProxyClientApply_JSONFallbackMarshalError verifies that a JSON marshal failure
// in the opaque fallback path is surfaced correctly.
func TestProxyClientApply_JSONFallbackMarshalError(t *testing.T) {
	skrClient, _ := newTestSKRClient(t)

	err := skrClient.Apply(t.Context(), &opaqueApplyConfig{marshalErr: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal apply configuration")
}

// Ensure ProxyClient satisfies client.Client at compile time.
var _ client.Client = (*skrclient.ProxyClient)(nil)
