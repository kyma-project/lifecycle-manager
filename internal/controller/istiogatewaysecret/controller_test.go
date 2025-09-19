package istiogatewaysecret_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kyma-project/lifecycle-manager/internal/controller/istiogatewaysecret"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
)

func TestReconcile_WhenGetSecretFuncReturnsError_ReturnError(t *testing.T) {
	// ARRANGE
	var stubGetterFunc istiogatewaysecret.GetterFunc = func(ctx context.Context,
		name types.NamespacedName,
	) (*apicorev1.Secret, error) {
		return nil, errors.New("some-error")
	}
	mockHandler := &mockHandler{}
	reconciler := istiogatewaysecret.NewReconciler(stubGetterFunc, mockHandler, queue.RequeueIntervals{})

	// ACT
	_, err := reconciler.Reconcile(t.Context(), ctrl.Request{})

	// ASSERT
	require.ErrorContains(t, err, "failed to get istio gateway root secret: some-error")
	assert.Equal(t, 0, mockHandler.calls)
}

func TestReconcile_WhenGetSecretFuncReturnsNoErrorAndSecretIsNil_ReturnError(t *testing.T) {
	// ARRANGE
	var stubGetterFunc istiogatewaysecret.GetterFunc = func(ctx context.Context,
		name types.NamespacedName,
	) (*apicorev1.Secret, error) {
		return nil, nil
	}
	mockHandler := &mockHandler{}
	reconciler := istiogatewaysecret.NewReconciler(stubGetterFunc, mockHandler, queue.RequeueIntervals{})

	// ACT
	_, err := reconciler.Reconcile(t.Context(), ctrl.Request{})

	// ASSERT
	require.ErrorIs(t, err, istiogatewaysecret.ErrSecretNotFound)
	assert.Equal(t, 0, mockHandler.calls)
}

func TestReconcile_WhenGetSecretFuncIsCalled_IsCalledWithRequestNamespacedName(t *testing.T) {
	// ARRANGE
	secretName, secretNamespace := "test-name", "test-namespace"
	request := ctrl.Request{NamespacedName: types.NamespacedName{Name: secretName, Namespace: secretNamespace}}

	var stubGetterFunc istiogatewaysecret.GetterFunc = func(ctx context.Context,
		name types.NamespacedName,
	) (*apicorev1.Secret, error) {
		assert.Equal(t, request.Namespace, name.Namespace)
		assert.Equal(t, request.Name, name.Name)
		return nil, nil
	}
	reconciler := istiogatewaysecret.NewReconciler(stubGetterFunc, &mockHandler{}, queue.RequeueIntervals{})

	// ACT
	// ASSERT
	_, _ = reconciler.Reconcile(t.Context(), request)
}

func TestReconcile_WhenGetSecretFuncReturnsSecret_HandlerManageGatewaySecretIsCalled(t *testing.T) {
	// ARRANGE
	secret := &apicorev1.Secret{}
	var stubGetterFunc istiogatewaysecret.GetterFunc = func(ctx context.Context,
		name types.NamespacedName,
	) (*apicorev1.Secret, error) {
		return secret, nil
	}
	mockHandler := &mockHandler{}
	reconciler := istiogatewaysecret.NewReconciler(stubGetterFunc, mockHandler, queue.RequeueIntervals{})

	// ACT
	_, err := reconciler.Reconcile(t.Context(), ctrl.Request{})

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, 1, mockHandler.calls)
}

func TestReconcile_WhenHandlerManageGatewaySecretReturnsError_ReturnError(t *testing.T) {
	// ARRANGE
	secret := &apicorev1.Secret{}
	var stubGetterFunc istiogatewaysecret.GetterFunc = func(ctx context.Context,
		name types.NamespacedName,
	) (*apicorev1.Secret, error) {
		return secret, nil
	}
	mockHandler := &mockHandler{err: errors.New("some-error")}
	reconciler := istiogatewaysecret.NewReconciler(stubGetterFunc, mockHandler, queue.RequeueIntervals{})

	// ACT
	_, err := reconciler.Reconcile(t.Context(), ctrl.Request{})

	// ASSERT
	require.ErrorContains(t, err, "failed to manage gateway secret: some-error")
	assert.Equal(t, 1, mockHandler.calls)
}

type mockHandler struct {
	calls int
	err   error
}

func (m *mockHandler) ManageGatewaySecret(_ context.Context, _ *apicorev1.Secret) error {
	m.calls++
	return m.err
}
