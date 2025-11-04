package skrsync_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrsync"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

func TestSyncImagePullSecret_WhenSecretNameIsNotConfigured_ReturnsError(t *testing.T) {
	skrSyncService := skrsync.NewService(nil, nil, nil, "")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.ErrorIs(t, err, skrsync.ErrImagePullSecretNotConfigured)
}

func TestSyncImagePullSecret_WhenSecretRepositoryReturnsError_ReturnsError(t *testing.T) {
	expectedError := errors.New("secret not found in repository")
	secretRepo := &secretRepositoryStub{err: expectedError}
	skrSyncService := skrsync.NewService(nil, secretRepo, nil, "test-secret")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.ErrorIs(t, err, skrsync.ErrImagePullSecretNotFound)
	require.ErrorIs(t, err, expectedError)
	require.True(t, secretRepo.called)
}

func TestSyncImagePullSecret_WhenSkrContextFactoryReturnsError_ReturnsError(t *testing.T) {
	expectedError := errors.New("failed to get SKR context")
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{Name: "test-secret"},
		Data:       map[string][]byte{"key": []byte("value")},
	}
	secretRepo := &secretRepositoryStub{secret: secret}
	skrContextFactory := &skrContextProviderStub{err: expectedError}
	skrSyncService := skrsync.NewService(skrContextFactory, secretRepo, nil, "test-secret")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.ErrorIs(t, err, expectedError)
	require.True(t, secretRepo.called)
	require.True(t, skrContextFactory.called)
}

func TestSyncImagePullSecret_WhenSkrContextPatchReturnsError_ReturnsError(t *testing.T) {
	patchError := errors.New("patch failed")
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:            "test-secret",
			ResourceVersion: "123",
			UID:             "uid-123",
		},
		Data: map[string][]byte{"key": []byte("value")},
	}
	secretRepo := &secretRepositoryStub{secret: secret}
	mockClient := &mockSkrClient{patchError: patchError}
	skrContextFactory := &skrContextProviderStub{mockClient: mockClient}
	skrSyncService := skrsync.NewService(skrContextFactory, secretRepo, nil, "test-secret")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.ErrorIs(t, err, skrsync.ErrFailedToSyncImagePullSecret)
	require.ErrorIs(t, err, patchError)
	require.True(t, secretRepo.called)
	require.True(t, skrContextFactory.called)
	require.True(t, mockClient.patchCalled)

	// Verify that the secret was properly prepared (metadata cleared)
	patchedSecret := mockClient.patchedObject.(*apicorev1.Secret)
	require.Equal(t, shared.DefaultRemoteNamespace, patchedSecret.Namespace)
	require.Empty(t, patchedSecret.ResourceVersion)
	require.Empty(t, patchedSecret.UID)
	require.Nil(t, patchedSecret.ManagedFields)
	require.Zero(t, patchedSecret.CreationTimestamp)
	require.Zero(t, patchedSecret.Generation)
}

func TestSyncImagePullSecret_WhenSuccessful_SyncsSecretToSkr(t *testing.T) {
	secret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:            "test-secret",
			Namespace:       "kcp-system",
			ResourceVersion: "123",
			UID:             "uid-123",
			Labels:          map[string]string{"label": "value"},
		},
		Data: map[string][]byte{"dockerconfigjson": []byte("config")},
	}
	secretRepo := &secretRepositoryStub{secret: secret}
	mockClient := &mockSkrClient{}
	skrContextFactory := &skrContextProviderStub{mockClient: mockClient}
	skrSyncService := skrsync.NewService(skrContextFactory, secretRepo, nil, "test-secret")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.NoError(t, err)
	require.True(t, secretRepo.called)
	require.True(t, skrContextFactory.called)
	require.True(t, mockClient.patchCalled)

	// Verify the secret was correctly modified before syncing
	patchedSecret := mockClient.patchedObject.(*apicorev1.Secret)
	require.Equal(t, "test-secret", patchedSecret.Name)
	require.Equal(t, shared.DefaultRemoteNamespace, patchedSecret.Namespace)
	require.Equal(t, map[string][]byte{"dockerconfigjson": []byte("config")}, patchedSecret.Data)
	require.Equal(t, map[string]string{"label": "value"}, patchedSecret.Labels)

	// Verify cluster-specific metadata was cleared
	require.Empty(t, patchedSecret.ResourceVersion)
	require.Empty(t, patchedSecret.UID)
	require.Nil(t, patchedSecret.ManagedFields)
	require.Zero(t, patchedSecret.CreationTimestamp)
	require.Zero(t, patchedSecret.Generation)

	// Verify patch options were correctly set
	require.Len(t, mockClient.patchOptions, 2)
	require.Contains(t, mockClient.patchOptions, client.ForceOwnership)
	require.Contains(t, mockClient.patchOptions, client.FieldOwner(shared.OperatorName))
}

func TestSyncCrds_WhenCalled_ShouldInvokeUseCase(t *testing.T) {
	syncCrdsUseCase := &syncCrdsUseCaseStub{}
	skrSyncService := skrsync.NewService(nil, nil, syncCrdsUseCase, "")

	_, err := skrSyncService.SyncCrds(t.Context(), &v1beta2.Kyma{})

	require.NoError(t, err)
	require.True(t, syncCrdsUseCase.called)
}

func TestSyncCrds_WhenUseCaseReturnsError_ReturnsError(t *testing.T) {
	expectedError := errors.New("usecase execution failed")
	syncCrdsUseCase := &syncCrdsUseCaseStub{err: expectedError}
	skrSyncService := skrsync.NewService(nil, nil, syncCrdsUseCase, "")

	result, err := skrSyncService.SyncCrds(t.Context(), &v1beta2.Kyma{})

	require.ErrorIs(t, err, expectedError)
	require.False(t, result)
	require.True(t, syncCrdsUseCase.called)
}

func TestSyncCrds_WhenUseCaseSucceeds_ReturnsResult(t *testing.T) {
	syncCrdsUseCase := &syncCrdsUseCaseStub{result: true}
	skrSyncService := skrsync.NewService(nil, nil, syncCrdsUseCase, "")

	result, err := skrSyncService.SyncCrds(t.Context(), &v1beta2.Kyma{})

	require.NoError(t, err)
	require.True(t, result)
	require.True(t, syncCrdsUseCase.called)
}

// Test Stubs

type syncCrdsUseCaseStub struct {
	called bool
	result bool
	err    error
}

func (s *syncCrdsUseCaseStub) Execute(_ context.Context, _ *v1beta2.Kyma) (bool, error) {
	s.called = true
	return s.result, s.err
}

type secretRepositoryStub struct {
	called bool
	secret *apicorev1.Secret
	err    error
}

func (s *secretRepositoryStub) Get(ctx context.Context, name string) (*apicorev1.Secret, error) {
	s.called = true
	return s.secret, s.err
}

type skrContextProviderStub struct {
	called     bool
	mockClient *mockSkrClient
	err        error
}

func (s *skrContextProviderStub) Get(kyma types.NamespacedName) (*remote.SkrContext, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	// Create a real SkrContext using our mock client
	return remote.NewSkrContext(s.mockClient, nil), nil
}

func (s *skrContextProviderStub) Init(ctx context.Context, kyma types.NamespacedName) error {
	return nil
}

func (s *skrContextProviderStub) InvalidateCache(kyma types.NamespacedName) {
}

// mockSkrClient implements remote.Client interface for testing
type mockSkrClient struct {
	client.Client

	patchCalled   bool
	patchedObject client.Object
	patchOptions  []client.PatchOption
	patchError    error
}

func (m *mockSkrClient) Patch(ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	m.patchCalled = true
	m.patchedObject = obj
	m.patchOptions = opts
	return m.patchError
}

func (m *mockSkrClient) Config() *rest.Config {
	return nil
}

// Implement minimal methods for client.Client interface that might be called
func (m *mockSkrClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (m *mockSkrClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (m *mockSkrClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (m *mockSkrClient) Get(ctx context.Context,
	key types.NamespacedName,
	obj client.Object,
	opts ...client.GetOption,
) error {
	return nil
}

func (m *mockSkrClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (m *mockSkrClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (m *mockSkrClient) Scheme() *runtime.Scheme {
	return nil
}

func (m *mockSkrClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (m *mockSkrClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *mockSkrClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
}

func (m *mockSkrClient) Status() client.SubResourceWriter {
	return nil
}

func (m *mockSkrClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}
