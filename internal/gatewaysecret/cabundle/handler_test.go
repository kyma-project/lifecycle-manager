package cabundle_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/cabundle"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/testutils"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/tests/fixtures/certificates"
)

const (
	gatewayServerCertSwitchGracePeriod = 90 * time.Minute
	gatewayServerCertExpiryWindow      = 14 * 24 * time.Hour
)

var (
	notBefore = time.Now().Add(-1 * time.Hour)
	notAfter  = time.Now().Add(2 * time.Hour)
)

// With the above notBefore and notAfter, the "new" CA cert is valid from 1 hour ago till the next two hours.
// In the good path, we bundle the CA cert immediately, since the "new" CA cert is already valid since 1 hour.
// And we will switch the TLS cert and key 30 minutes from now, since the grace period is 90 minutes from the notBefore.

type noopMetrics struct{}

func (noopMetrics) ServerCertificateCloseToExpiry(_ bool) {}

func TestManageGatewaySecret_WhenGetWatcherServingCertValidityReturnsError_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	someError := errors.New("some-error")
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(time.Time{}, time.Time{}, someError)

	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
				//nolint:revive // false positive
				string(secret.Data[apicorev1.TLSPrivateKeyKey]) == string(rootSecret.Data[apicorev1.TLSPrivateKeyKey]) &&
				string(secret.Data["ca.crt"]) == string(rootSecret.Data["ca.crt"]) &&
				secret.Annotations[shared.CaAddedToBundleAtAnnotation] != ""
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
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		},
	}, nil)
	expectedError := errors.New("some-error")
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(expectedError)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)
	expiredServerCertSwitchGracePeriod := 30 * time.Minute
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		expiredServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(assert.AnError)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
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
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(assert.AnError)
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(certificate.WithParseX509Function(
			func(_ []byte) (*x509.Certificate, error) {
				return &x509.Certificate{
					NotAfter: time.Time{},
				}, nil
			},
		)),
		noopMetrics{},
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

func TestManageGatewaySecret_WhenServerCertCloseToExpiry_SetsMetricToTrue(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	// tls.crt expires in 7 days, within the 14-day expiry window
	expiringCert := generateCertPEM(t, time.Now().Add(-time.Hour), time.Now().Add(7*24*time.Hour))
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       expiringCert,
			apicorev1.TLSPrivateKeyKey: []byte("key"),
			"ca.crt":                   certificates.Cert1,
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)

	var gotSet bool
	metricsMock := metricsMockFunc(func(set bool) { gotSet = set })
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		metricsMock,
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("key"),
			"ca.crt":                   certificates.Cert1,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	assert.True(t, gotSet, "expected metric to be set to true for expiring cert")
}

func TestManageGatewaySecret_WhenServerCertNotCloseToExpiry_SetsMetricToFalse(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	// Cert1 is valid until 2036, well outside the 14-day window
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("key"),
			"ca.crt":                   certificates.Cert1,
		},
	}, nil)
	mockClient.On("UpdateGatewaySecret", mock.Anything, mock.Anything).Return(nil)

	var gotSet bool
	metricsMock := metricsMockFunc(func(set bool) { gotSet = set })
	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		metricsMock,
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert2,
			apicorev1.TLSPrivateKeyKey: []byte("key"),
			"ca.crt":                   certificates.Cert2,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.NoError(t, err)
	assert.False(t, gotSet, "expected metric to be set to false for non-expiring cert")
}

func TestManageGatewaySecret_WhenServerCertIsInvalidPEM_ReturnsError(t *testing.T) {
	// ARRANGE
	mockClient := &testutils.ClientMock{}
	mockClient.On("GetWatcherServingCertValidity", mock.Anything).Return(notBefore, notAfter, nil)
	mockClient.On("GetGatewaySecret", mock.Anything).Return(&apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       []byte("not-valid-pem"),
			apicorev1.TLSPrivateKeyKey: []byte("key"),
			"ca.crt":                   certificates.Cert1,
		},
	}, nil)

	handler := cabundle.NewGatewaySecretHandler(mockClient,
		gatewayServerCertSwitchGracePeriod,
		gatewayServerCertExpiryWindow,
		certificate.NewBundler(),
		noopMetrics{},
	)
	rootSecret := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       certificates.Cert1,
			apicorev1.TLSPrivateKeyKey: []byte("key"),
			"ca.crt":                   certificates.Cert1,
		},
	}

	// ACT
	err := handler.ManageGatewaySecret(t.Context(), rootSecret)

	// ASSERT
	require.ErrorIs(t, err, cabundle.ErrorServerCertificateParsingFailure)
	mockClient.AssertNumberOfCalls(t, "UpdateGatewaySecret", 0)
}

type metricsMockFunc func(bool)

func (f metricsMockFunc) ServerCertificateCloseToExpiry(set bool) { f(set) }

func generateCertPEM(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func notFoundError() error {
	return apierrors.NewNotFound(apicorev1.Resource("secrets"), "not-found")
}
