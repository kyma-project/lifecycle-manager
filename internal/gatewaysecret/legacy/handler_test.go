package legacy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/legacy"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/testutils"
)

var notBefore = time.Now().Add(time.Hour)

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	someError := errors.New("some-error")
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, someError)

	handler := legacy.NewGatewaySecretHandler(mockClient, nil)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, someError)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundError_CreatesGatewaySecretFromRootSecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}
	handler := legacy.NewGatewaySecretHandler(mockClient, nil)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "CreateGatewaySecret", 1)

	expectedNamespace := "istio-system"
	expectedName := "klm-istio-gateway"
	mockClient.AssertCalled(t, "CreateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return secret.Name == expectedName &&
				secret.Namespace == expectedNamespace &&
				string(secret.Data["tls.crt"]) == string(rootSecret.Data["tls.crt"]) &&
				string(secret.Data["tls.key"]) == string(rootSecret.Data["tls.key"]) &&
				string(secret.Data["ca.crt"]) == string(rootSecret.Data["ca.crt"])
		}))
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundErrorAndCreationFailed_ReturnError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	expectedError := errors.New("some-error")
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)

	handler := legacy.NewGatewaySecretHandler(mockClient, nil)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "CreateGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundError_CreatesGatewaySecretWithLastModifiedAnnotation(
	t *testing.T,
) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(nil)

	handler := legacy.NewGatewaySecretHandler(mockClient, nil)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "CreateGatewaySecret", 1)
	mockClient.AssertCalled(t, "CreateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return secret.Annotations[shared.LastModifiedAtAnnotation] != ""
		}))
}

func TestManageGatewaySecret_WhenGetWatcherServingCertValidityReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{}, nil)
	expectedError := errors.New("some-error")
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Time{}, time.Time{}, expectedError)

	handler := legacy.NewGatewaySecretHandler(mockClient, nil)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
}

func TestManageGatewaySecret_WhenRequiresUpdate_UpdatesGatewaySecretWithRootSecretData(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{}, nil)
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, time.Time{}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		return time.Now(), nil
	}
	handler := legacy.NewGatewaySecretHandler(mockClient, mockFunc)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return string(secret.Data["tls.crt"]) == string(rootSecret.Data["tls.crt"]) &&
				string(secret.Data["tls.key"]) == string(rootSecret.Data["tls.key"]) &&
				string(secret.Data["ca.crt"]) == string(rootSecret.Data["ca.crt"])
		}))
}

func TestManageGatewaySecret_WhenRequiresUpdate_UpdatesGatewaySecretWithUpdatedModifiedNowAnnotation(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	originalTime := time.Now().Add(-time.Hour)
	gwSecret := &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: originalTime.Format(time.RFC3339),
			},
		},
	}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(gwSecret, nil)
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, time.Time{}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		return time.Now(), nil
	}
	handler := legacy.NewGatewaySecretHandler(mockClient, mockFunc)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			lastModified, ok := secret.Annotations[shared.LastModifiedAtAnnotation]
			if !ok {
				return false
			}
			lastModifiedTime, _ := time.Parse(time.RFC3339, lastModified)

			return lastModifiedTime.After(originalTime)
		}))
}

func TestManageGatewaySecret_WhenRequiresUpdateAndUpdateFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{}, nil)
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, time.Time{}, nil)
	expectedError := errors.New("some-error")
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		return time.Now(), nil
	}
	handler := legacy.NewGatewaySecretHandler(mockClient, mockFunc)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenRequiresUpdateIsFalse_DoesNotUpdateGatewaySecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{}, nil)
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Now().Add(-1*time.Hour), time.Time{}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		return time.Now(), nil
	}
	handler := legacy.NewGatewaySecretHandler(mockClient, mockFunc)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 0)
}

func TestManageGatewaySecret_WhenTimeParserFuncReturnsError_UpdatesGatewaySecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{}, nil)
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, time.Time{}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		return time.Time{}, errors.New("some-error")
	}
	handler := legacy.NewGatewaySecretHandler(mockClient, mockFunc)

	// ACT″
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
}

func notFoundError() error {
	return apierrors.NewNotFound(apicorev1.Resource("secrets"), "not-found")
}
