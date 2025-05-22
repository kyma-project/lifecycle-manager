package cabundle_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/cabundle"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/testutils"
)

const gatewaySwitchCertBeforeExpirationTime = 1 * time.Hour

var (
	notBefore = time.Now().Add(-1 * time.Hour)
	notAfter  = time.Now().Add(2 * time.Hour)
)

func TestManageGatewaySecret_WhenGetWatcherServingCertValidityReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	someError := errors.New("some-error")
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Time{}, time.Time{}, someError)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, gatewaySwitchCertBeforeExpirationTime)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, someError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
}

func TestManageGatewaySecret_WhenGetWatcherServingCertValidityReturnsInvalidNotBefore_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Time{}, time.Now(), nil)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, gatewaySwitchCertBeforeExpirationTime)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, cabundle.ErrCACertificateNotReady)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
}

func TestManageGatewaySecret_WhenGetWatcherServingCertValidityReturnsInvalidNotAfter_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Now(), time.Time{}, nil)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, gatewaySwitchCertBeforeExpirationTime)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, cabundle.ErrCACertificateNotReady)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	someError := errors.New("some-error")
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, someError)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, gatewaySwitchCertBeforeExpirationTime)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, someError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundError_CreatesGatewaySecretFromRootSecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, gatewaySwitchCertBeforeExpirationTime)

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
				string(secret.Data["ca.crt"]) == string(rootSecret.Data["ca.crt"]) &&
				string(secret.Data["temp.ca.crt"]) == string(rootSecret.Data["ca.crt"]) &&
				secret.Annotations[shared.LastModifiedAtAnnotation] != "" &&
				secret.Annotations[cabundle.CurrentCAExpirationAnnotation] != ""
		}))
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundErrorAndCreationFailed_ReturnError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	expectedError := errors.New("some-error")
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, gatewaySwitchCertBeforeExpirationTime)

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "CreateGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenLegacySecret_BootstrapsLegacyGatewaySecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	timeParserFunction := getTimeParserFunction(false, false)
	handler := cabundle.NewGatewaySecretHandler(mockClient, timeParserFunction, gatewaySwitchCertBeforeExpirationTime)
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
			return secret.Annotations[cabundle.CurrentCAExpirationAnnotation] != "" &&
				string(secret.Data["temp.ca.crt"]) == "value3"
		}))
}

//nolint:dupl // the tests may contain similar code but they test different scenarios
func TestManageGatewaySecret_WhenRequiresBundling_BundlesGatewaySecretWithRootSecretCA(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt":     []byte("value1"),
			"tls.key":     []byte("value2"),
			"ca.crt":      []byte("value3"),
			"temp.ca.crt": []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	timeParserFunction := getTimeParserFunction(true, false)
	handler := cabundle.NewGatewaySecretHandler(mockClient, timeParserFunction, gatewaySwitchCertBeforeExpirationTime)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return string(secret.Data["tls.crt"]) == "value1" &&
				string(secret.Data["tls.key"]) == "value2" &&
				string(secret.Data["ca.crt"]) == "new-value3value3" &&
				string(secret.Data["temp.ca.crt"]) == "new-value3"
		}))
}

func TestManageGatewaySecret_WhenUpdateSecretFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt":     []byte("value1"),
			"tls.key":     []byte("value2"),
			"ca.crt":      []byte("value3"),
			"temp.ca.crt": []byte("value3"),
		},
	}, nil)
	expectedError := errors.New("some-error")
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)
	timeParserFunction := getTimeParserFunction(true, false)
	handler := cabundle.NewGatewaySecretHandler(mockClient, timeParserFunction, gatewaySwitchCertBeforeExpirationTime)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCertValidity", 1)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
}

//nolint:dupl // the tests may contain similar code but they test different scenarios
func TestManageGatewaySecret_WhenRequiresCertSwitching_SwitchesTLSCertAndKeyWithRootSecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt":     []byte("value1"),
			"tls.key":     []byte("value2"),
			"ca.crt":      []byte("new-value3value3"),
			"temp.ca.crt": []byte("new-value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	timeParserFunction := getTimeParserFunction(false, true)
	handler := cabundle.NewGatewaySecretHandler(mockClient, timeParserFunction, gatewaySwitchCertBeforeExpirationTime)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return string(secret.Data["tls.crt"]) == "new-value1" &&
				string(secret.Data["tls.key"]) == "new-value2" &&
				string(secret.Data["ca.crt"]) == "new-value3value3" &&
				string(secret.Data["temp.ca.crt"]) == "new-value3"
		}))
}

func getTimeParserFunction(bundlingRequired, certSwitchRequired bool) gatewaysecret.TimeParserFunc {
	var lastModifiedAt, currentCAExpiration time.Time

	if bundlingRequired {
		lastModifiedAt = time.Now().Add(-2 * time.Hour)
	} else {
		lastModifiedAt = time.Now()
	}
	if certSwitchRequired {
		currentCAExpiration = time.Now().Add(30 * time.Minute)
	} else {
		currentCAExpiration = time.Now().Add(2 * time.Hour)
	}

	return func(secret *apicorev1.Secret, annotation string) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return lastModifiedAt, nil
		}
		return currentCAExpiration, nil
	}
}

func notFoundError() error {
	return apierrors.NewNotFound(apicorev1.Resource("secrets"), "not-found")
}
