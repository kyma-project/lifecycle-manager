package cabundle_test

import (
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/cabundle"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/testutils"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/tests/fixtures/certificates"
)

const gatewayServerCertSwitchGracePeriod = 90 * time.Minute

var (
	notBefore = time.Now().Add(-1 * time.Hour)
	notAfter  = time.Now().Add(2 * time.Hour)
)

// With the above notBefore and notAfter, the "new" CA cert is valid from 1 hour ago till the next two hours.
// In the good path, we bundle the CA cert immediately, since the "new" CA cert is already valid since 1 hour.
// And we will switch the TLS cert and key 30 minutes from now, since the grace period is 90 minutes from the notBefore.

func TestManageGatewaySecret_WhenGetWatcherServingCertValidityReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	someError := errors.New("some-error")
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Time{}, time.Time{}, someError)

	handler := cabundle.NewGatewaySecretHandler(mockClient,
		nil,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)

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

	handler := cabundle.NewGatewaySecretHandler(mockClient,
		nil,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)

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

	handler := cabundle.NewGatewaySecretHandler(mockClient,
		nil,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)

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
			apicorev1.TLSCertKey:       []byte("value1"),
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   []byte("value3"),
		},
	}
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		nil,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)

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
				string(secret.Data[apicorev1.TLSCertKey]) == string(rootSecret.Data[apicorev1.TLSCertKey]) &&
				string(secret.Data[apicorev1.TLSPrivateKeyKey]) == string(rootSecret.Data[apicorev1.TLSPrivateKeyKey]) &&
				string(secret.Data["ca.crt"]) == string(rootSecret.Data["ca.crt"]) &&
				string(secret.Data["temp.ca.crt"]) == string(rootSecret.Data["ca.crt"]) &&
				secret.Annotations[shared.LastModifiedAtAnnotation] != ""
		}))
}

func TestManageGatewaySecret_WhenGetGatewaySecretReturnsNotFoundErrorAndCreationFailed_ReturnError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(nil, notFoundError())
	expectedError := errors.New("some-error")
	mockClient.On("CreateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)

	handler := cabundle.NewGatewaySecretHandler(mockClient,
		nil,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)

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
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   certificates.Cert1,
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	timeParserFunction := getTimeParserFunction(false)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		timeParserFunction,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   certificates.Cert2,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
}

func TestManageGatewaySecret_BundlesAndDropsCerts(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   append(certificates.Cert1, certificates.CertExpired...),
			"temp.ca.crt":              []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	timeParserFunction := getTimeParserFunction(true)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		timeParserFunction,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("new-value2"),
			"ca.crt":                   certificates.Cert2,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return string(secret.Data[apicorev1.TLSCertKey]) == string(certificates.Cert1) &&
				string(secret.Data[apicorev1.TLSPrivateKeyKey]) == "value2" &&
				string(secret.Data["ca.crt"]) == string(append(certificates.Cert2, certificates.Cert1...))
		}))
}

func TestManageGatewaySecret_WhenUpdateSecretFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   certificates.Cert1,
			"temp.ca.crt":              []byte("value3"),
		},
	}, nil)
	expectedError := errors.New("some-error")
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)
	timeParserFunction := getTimeParserFunction(true)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		timeParserFunction,
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("new-value2"),
			"ca.crt":                   certificates.Cert2,
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

func TestManageGatewaySecret_WhenRequiresCertSwitching_SwitchesTLSCertAndKeyWithRootSecret(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   certificates.Cert2,
			"temp.ca.crt":              []byte("new-value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	timeParserFunction := getTimeParserFunction(false)
	expiredServerCertSwitchGracePeriod := 30 * time.Minute
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		timeParserFunction,
		expiredServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("new-value2"),
			"ca.crt":                   certificates.Cert2,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 1)
	mockClient.AssertCalled(t, "UpdateGatewaySecret", mock.Anything, mock.MatchedBy(
		func(secret *apicorev1.Secret) bool {
			return string(secret.Data[apicorev1.TLSCertKey]) == string(certificates.Cert2) &&
				string(secret.Data[apicorev1.TLSPrivateKeyKey]) == "new-value2" &&
				string(secret.Data["ca.crt"]) == string(certificates.Cert2)
		}))
}

func TestManageGatewaySecret_WhenBundlingFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   []byte("invalid bundle"),
			"temp.ca.crt":              []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(assert.AnError)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		getTimeParserFunction(false),
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(),
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("new-value2"),
			"ca.crt":                   certificates.Cert2,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.ErrorIs(t, err, certificate.ErrInvalidPEM)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 0)
}

func TestManageGatewaySecret_WhenDroppingExpiredCertsFails_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("value2"),
			"ca.crt":                   certificates.Cert1,
			"temp.ca.crt":              []byte("value3"),
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(assert.AnError)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		getTimeParserFunction(false),
		gatewayServerCertSwitchGracePeriod,
		certificate.NewBundler(certificate.WithParseX509Function(
			func(_ []byte) (*x509.Certificate, error) {
				return &x509.Certificate{
					NotAfter: time.Time{},
				}, nil
			},
		)),
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("new-value2"),
			"ca.crt":                   certificates.Cert2,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.ErrorIs(t, err, certificate.ErrX509NotAfterIsZero)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 0)
}

func getTimeParserFunction(bundlingRequired bool) gatewaysecret.TimeParserFunc {
	var lastModifiedAt time.Time

	if bundlingRequired {
		lastModifiedAt = time.Now().Add(-2 * time.Hour)
	} else {
		lastModifiedAt = time.Now()
	}

	return func(secret *apicorev1.Secret, annotation string) (time.Time, error) {
		if annotation == shared.LastModifiedAtAnnotation {
			return lastModifiedAt, nil
		}
		return time.Time{}, nil
	}
}

func notFoundError() error {
	return apierrors.NewNotFound(apicorev1.Resource("secrets"), "not-found")
}
