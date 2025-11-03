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
	skrContextFactory := &skrContextFactoryStub{err: expectedError}
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
	skrContext := &skrContextStub{patchError: patchError}
	skrContextFactory := &skrContextFactoryStub{skrContext: skrContext}
	skrSyncService := skrsync.NewService(skrContextFactory, secretRepo, nil, "test-secret")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.ErrorIs(t, err, skrsync.ErrFailedToSyncImagePullSecret)
	require.ErrorIs(t, err, patchError)
	require.True(t, secretRepo.called)
	require.True(t, skrContextFactory.called)
	require.True(t, skrContext.patchCalled)

	// Verify that the secret was properly prepared (metadata cleared)
	patchedSecret := skrContext.patchedObject.(*apicorev1.Secret)
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
	skrContext := &skrContextStub{}
	skrContextFactory := &skrContextFactoryStub{skrContext: skrContext}
	skrSyncService := skrsync.NewService(skrContextFactory, secretRepo, nil, "test-secret")

	err := skrSyncService.SyncImagePullSecret(t.Context(), random.NamespacedName())

	require.NoError(t, err)
	require.True(t, secretRepo.called)
	require.True(t, skrContextFactory.called)
	require.True(t, skrContext.patchCalled)

	// Verify the secret was correctly modified before syncing
	patchedSecret := skrContext.patchedObject.(*apicorev1.Secret)
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

	// Verify patch options
	require.Len(t, skrContext.patchOptions, 2)
	require.Contains(t, skrContext.patchOptions, client.ForceOwnership)
	require.Contains(t, skrContext.patchOptions, client.FieldOwner(shared.OperatorName))
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

func (s *secretRepositoryStub) Get(_ context.Context, _ string) (*apicorev1.Secret, error) {
	s.called = true
	return s.secret, s.err
}

type skrContextFactoryStub struct {
	called     bool
	skrContext *skrContextStub
	err        error
}

func (s *skrContextFactoryStub) Get(_ types.NamespacedName) (*remote.SkrContext, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	clientStub := &clientStub{
		patchCalled:   &s.skrContext.patchCalled,
		patchedObject: &s.skrContext.patchedObject,
		patchOptions:  &s.skrContext.patchOptions,
		patchError:    s.skrContext.patchError,
	}
	return remote.NewSkrContext(clientStub, nil), nil
}

func (s *skrContextFactoryStub) Init(ctx context.Context, kyma types.NamespacedName) error {
	return nil
}

func (s *skrContextFactoryStub) InvalidateCache(kyma types.NamespacedName) {
}

type skrContextStub struct {
	patchCalled   bool
	patchedObject client.Object
	patchOptions  []client.PatchOption
	patchError    error
}

func (s *skrContextStub) Patch(ctx context.Context,
	obj client.Object,
	opts ...client.PatchOption,
) error {
	s.patchCalled = true
	s.patchedObject = obj
	s.patchOptions = opts
	return s.patchError
}

func (s *skrContextStub) Create(_ context.Context, _ client.Object, _ ...client.CreateOption) error {
	return nil
}

func (s *skrContextStub) Update(_ context.Context, _ client.Object, _ ...client.UpdateOption) error {
	return nil
}

func (s *skrContextStub) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return nil
}

func (s *skrContextStub) Get(_ context.Context,
	_ types.NamespacedName,
	_ client.Object,
	_ ...client.GetOption,
) error {
	return nil
}

func (s *skrContextStub) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return nil
}

func (s *skrContextStub) DeleteAllOf(_ context.Context, _ client.Object, _ ...client.DeleteAllOfOption) error {
	return nil
}

func (s *skrContextStub) Scheme() *runtime.Scheme {
	return nil
}

func (s *skrContextStub) RESTMapper() meta.RESTMapper {
	return nil
}

func (s *skrContextStub) GroupVersionKindFor(_ runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (s *skrContextStub) IsObjectNamespaced(_ runtime.Object) (bool, error) {
	return false, nil
}

func (s *skrContextStub) Status() client.SubResourceWriter {
	return nil
}

func (s *skrContextStub) SubResource(_ string) client.SubResourceClient {
	return nil
}

type clientStub struct {
	client.Client

	patchCalled   *bool
	patchedObject *client.Object
	patchOptions  *[]client.PatchOption
	patchError    error
}

func (c *clientStub) Patch(_ context.Context,
	obj client.Object,
	opts ...client.PatchOption,
) error {
	*c.patchCalled = true
	*c.patchedObject = obj
	*c.patchOptions = opts
	return c.patchError
}

func (c *clientStub) Config() *rest.Config {
	return nil
}
