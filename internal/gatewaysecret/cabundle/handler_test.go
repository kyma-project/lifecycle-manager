package cabundle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/cabundle"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/testutils"
)

func TestManageGatewaySecret_WhenGetWatcherServingCertReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	someError := errors.New("some-error")
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(nil, someError)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, 1*time.Hour)

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, someError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCert", 1)
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
	someError := errors.New("some-error")
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, someError)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, 1*time.Hour)

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, someError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCert", 1)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundError_CreatesGatewaySecretFromRootSecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, 1*time.Hour)

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "CreateGatewaySecret", 1)

	expectedNamespace := "istio-system"
	expectedName := "klm-istio-gateway"
	mockClient.AssertCalled(t, "CreateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return secret.ObjectMeta.Name == expectedName &&
				secret.ObjectMeta.Namespace == expectedNamespace &&
				string(secret.Data["tls.crt"]) == string(rootSecret.Data["tls.crt"]) &&
				string(secret.Data["tls.key"]) == string(rootSecret.Data["tls.key"]) &&
				string(secret.Data["ca.crt"]) == string(rootSecret.Data["ca.crt"]) &&
				secret.Annotations[shared.LastModifiedAtAnnotation] != ""
		}))
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundErrorAndCreationFailed_ReturnError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	expectedError := errors.New("some-error")
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)

	handler := cabundle.NewGatewaySecretHandler(mockClient, nil, 1*time.Hour)

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), &apicorev1.Secret{})

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetGatewaySecret", 1)
	mockClient.AssertNumberOfCalls(t, "CreateGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenLegacySecret_BootstrapsLegacyGatewaySecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(-time.Hour),
			},
			NotAfter: &apimetav1.Time{
				Time: time.Now().Add(2 * time.Hour),
			},
		},
	}, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return time.Now(), nil // bundling required
		} // bundling not required
		return time.Now().Add(2 * time.Hour), nil // cert switching not required
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, mockFunc, 1*time.Hour)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("value1"),
			"tls.key": []byte("value2"),
			"ca.crt":  []byte("value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return secret.Annotations[cabundle.CurrentCAExpirationAnnotation] != "" &&
				string(secret.Data["temp.ca.crt"]) == "value3"
		}))
}

func TestManageGatewaySecret_WhenRequiresBundling_BundlesGatewaySecretWithRootSecretCA(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt":     []byte("value1"),
			"tls.key":     []byte("value2"),
			"ca.crt":      []byte("value3"),
			"temp.ca.crt": []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return time.Now(), nil // bundling required
		}
		return time.Now().Add(2 * time.Hour), nil // cert switching not required
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, mockFunc, 1*time.Hour)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), rootSecret)

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

func TestManageGatewaySecret_WhenRequiresBundlingAndUpdateFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
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
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return time.Now(), nil // bundling required
		}
		return time.Now().Add(2 * time.Hour), nil // cert switching not required
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, mockFunc, 1*time.Hour)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), rootSecret)

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCert", 1)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
}

func TestManageGatewaySecret_WhenRequiresCertSwitching_SwitchesTLSCertAndKeyWithRootSecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt":     []byte("value1"),
			"tls.key":     []byte("value2"),
			"ca.crt":      []byte("new-value3value3"),
			"temp.ca.crt": []byte("new-value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return time.Now().Add(2 * time.Hour), nil // bundling not required
		}
		return time.Now().Add(30 * time.Minute), nil // cert switching required
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, mockFunc, 1*time.Hour)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), rootSecret)

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

func TestManageGatewaySecret_WhenRequiresCertSwitchingAndUpdateFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCert", mock.Anything).Return(&certmanagerv1.Certificate{
		Status: certmanagerv1.CertificateStatus{
			NotBefore: &apimetav1.Time{
				Time: time.Now().Add(time.Hour),
			},
		},
	}, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt":     []byte("value1"),
			"tls.key":     []byte("value2"),
			"ca.crt":      []byte("new-value3value3"),
			"temp.ca.crt": []byte("new-value3"),
		},
	}, nil)
	expectedError := errors.New("some-error")
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)
	var mockFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret,
		annotation string,
	) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return time.Now().Add(2 * time.Hour), nil // bundling not required
		}
		return time.Now().Add(30 * time.Minute), nil // cert switching required
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient, mockFunc, 1*time.Hour)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("new-value1"),
			"tls.key": []byte("new-value2"),
			"ca.crt":  []byte("new-value3"),
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(context.TODO(), rootSecret)

	// ASSERT
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
	mockClient.AssertNumberOfCalls(t, "GetWatcherServingCert", 1)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
}

func notFoundError() error {
	return apierrors.NewNotFound(apicorev1.Resource("secrets"), "not-found")
}
