package certificate_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/secret/data"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
)

var (
	kymaName               = random.Name()
	expectedCertNameSuffix = "-webhook-tls"
	certNamespace          = random.Name()
	gatewaySecretName      = random.Name()
	skrServiceName         = random.Name()
	skrNamespace           = random.Name()
	additionalDNSNames     = []string{
		random.Name(),
		random.Name(),
	}
	skrDomainName = random.Name()
	runtimeID     = random.Name()

	renewBuffer = 10 * time.Minute
)

func TestCreateSkrCertificate_Success(t *testing.T) {
	certRepo := &certRepoStub{}
	certService := certificate.NewService(certRepo, &secretRepoStub{}, certificate.Config{
		AdditionalDNSNames: additionalDNSNames,
		SkrServiceName:     skrServiceName,
		SkrNamespace:       skrNamespace,
	})
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
			Annotations: map[string]string{
				shared.SkrDomainAnnotation: skrDomainName,
			},
			Labels: map[string]string{
				shared.RuntimeIDLabel: runtimeID,
			},
		},
	}

	err := certService.CreateSkrCertificate(t.Context(), kyma)

	require.NoError(t, err)
	require.True(t, certRepo.createCalled)
	require.Equal(t, runtimeID, certRepo.createCommonName)
	require.Equal(t, kymaName+expectedCertNameSuffix, certRepo.createName)
	require.Contains(t, certRepo.createDNSNames, skrDomainName)
	require.Contains(t, certRepo.createDNSNames, additionalDNSNames[0])
	require.Contains(t, certRepo.createDNSNames, additionalDNSNames[1])
	require.Contains(t, certRepo.createDNSNames,
		fmt.Sprintf("%s.%s.svc.cluster.local", skrServiceName, skrNamespace))
	require.Contains(t, certRepo.createDNSNames, fmt.Sprintf("%s.%s.svc", skrServiceName, skrNamespace))
}

func TestCreateSkrCertificate_CertificateRepositoryReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		createErr: assert.AnError,
	}
	certService := certificate.NewService(certRepo, &secretRepoStub{}, certificate.Config{
		AdditionalDNSNames: additionalDNSNames,
	})
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
			Annotations: map[string]string{
				shared.SkrDomainAnnotation: skrDomainName,
			},
		},
	}

	err := certService.CreateSkrCertificate(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to create SKR certificate")
	require.True(t, certRepo.createCalled)
}

func TestCreateSkrCertificate_ErrDomainAnnotationMissing_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{}
	certService := certificate.NewService(certRepo, &secretRepoStub{}, certificate.Config{
		AdditionalDNSNames: additionalDNSNames,
	})
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
		},
	}

	err := certService.CreateSkrCertificate(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, certificate.ErrDomainAnnotationMissing)
	require.ErrorContains(t, err, "failed to construct DNS names")
	require.False(t, certRepo.createCalled)
}

func TestCreateSkrCertificate_ErrDomainAnnotationEmpty_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{}
	certService := certificate.NewService(certRepo, &secretRepoStub{}, certificate.Config{
		AdditionalDNSNames: additionalDNSNames,
	})
	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
			Annotations: map[string]string{
				shared.SkrDomainAnnotation: "",
			},
		},
	}

	err := certService.CreateSkrCertificate(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, certificate.ErrDomainAnnotationEmpty)
	require.ErrorContains(t, err, "failed to construct DNS names")
	require.False(t, certRepo.createCalled)
}

func TestDeleteSkrCertificate_Success(t *testing.T) {
	certRepo := &certRepoStub{}
	secretRepo := &secretRepoStub{}
	certService := certificate.NewService(certRepo, secretRepo, certificate.Config{})

	err := certService.DeleteSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	require.True(t, certRepo.deleteCalled)
	require.Equal(t, kymaName+expectedCertNameSuffix, certRepo.deleteName)
	require.True(t, secretRepo.deleteCalled)
	require.Equal(t, kymaName+expectedCertNameSuffix, secretRepo.deleteName)
}

func TestDeleteSkrCertificate_CertificateRepositoryReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		deleteErr: assert.AnError,
	}
	secretRepo := &secretRepoStub{}
	certService := certificate.NewService(certRepo, secretRepo, certificate.Config{})

	err := certService.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to delete SKR certificate")
	require.True(t, certRepo.deleteCalled)
	require.False(t, secretRepo.deleteCalled)
}

func TestDeleteSkrCertificate_SecretRepositoryReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{}
	secretRepo := &secretRepoStub{
		deleteErr: assert.AnError,
	}
	certService := certificate.NewService(certRepo, secretRepo, certificate.Config{})

	err := certService.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to delete SKR certificate secret")
	require.True(t, certRepo.deleteCalled)
	require.True(t, secretRepo.deleteCalled)
}

func TestIsSkrCertificateRenewalOverdue_WhenRenewalTimeMatches_ReturnsTrue(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-time.Hour),
		getValidityEnd:   time.Now().Add(time.Hour),
	}
	gatewaySecret := getGatewaySecretWithLastModifiedAnnotation(time.Now().Format(time.RFC3339))
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{gatewaySecret, gatewaySecret},
	}
	certService := certificate.NewService(certRepo, secretRepo, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := certService.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	require.True(t, overdue)
	require.True(t, certRepo.getValidityCalled)
	require.True(t, secretRepo.getCalled)
}

func getGatewaySecretWithLastModifiedAnnotation(lastModifiedAnnotation string) *apicorev1.Secret {
	return &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      gatewaySecretName,
			Namespace: testutils.IstioNamespace,
			Annotations: map[string]string{
				shared.LastModifiedAtAnnotation: lastModifiedAnnotation,
			},
		},
	}
}

func TestIsSkrCertificateRenewalOverdue_ReturnsFalse(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(time.Second),
		getValidityEnd:   time.Now().Add(time.Hour),
	}
	gatewaySecret := getGatewaySecretWithLastModifiedAnnotation(time.Now().Format(time.RFC3339))
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{gatewaySecret},
	}
	certService := certificate.NewService(certRepo, secretRepo, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := certService.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	require.False(t, overdue)
	require.True(t, certRepo.getValidityCalled)
	require.True(t, secretRepo.getCalled)
}

func TestIsSkrCertificateRenewalOverdue_CertRepositoryReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityErr: assert.AnError,
	}
	certService := certificate.NewService(certRepo, &secretRepoStub{}, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := certService.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to determine if SKR certificate needs renewal")
	require.True(t, overdue)
	require.True(t, certRepo.getValidityCalled)
}

func TestIsSkrCertificateRenewalOverdue_SecretRepositoryReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepoStub{
		getValidityStart: time.Now().Add(-time.Hour),
		getValidityEnd:   time.Now().Add(time.Hour),
	}
	secretRepo := &secretRepoStub{
		getErrors: []error{assert.AnError},
	}
	certService := certificate.NewService(certRepo, secretRepo, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := certService.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to determine gateway secret lastModifiedAt")
	require.True(t, overdue)
	require.True(t, certRepo.getValidityCalled)
	require.True(t, secretRepo.getCalled)
}

func TestGetSkrCertificateSecret_SecretRepositoryReturns_Success(t *testing.T) {
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      kymaName + expectedCertNameSuffix,
					Namespace: certNamespace,
				},
			},
		},
	}
	certService := certificate.NewService(&certRepoStub{}, secretRepo, certificate.Config{})

	result, err := certService.GetSkrCertificateSecret(t.Context(), kymaName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, kymaName+expectedCertNameSuffix, result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func TestGetSkrCertificateSecret_SecretRepositoryReturnsError_ReturnsError(t *testing.T) {
	secretRepo := &secretRepoStub{
		getErrors: []error{assert.AnError},
	}
	certService := certificate.NewService(&certRepoStub{}, secretRepo, certificate.Config{})

	result, err := certService.GetSkrCertificateSecret(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to get SKR certificate secret")
	assert.Nil(t, result)
	assert.True(t, secretRepo.getCalled)
}

func TestGetSkrCertificateSecret_SecretRepositoryReturnsNotFound_ReturnsError(t *testing.T) {
	secretRepo := &secretRepoStub{
		getErrors: []error{
			apierrors.NewNotFound(apicorev1.Resource("secrets"), kymaName+expectedCertNameSuffix),
		},
	}
	certService := certificate.NewService(&certRepoStub{}, secretRepo, certificate.Config{})

	result, err := certService.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	require.ErrorContains(t, err, "failed to get SKR certificate secret")
	assert.Nil(t, result)
	assert.True(t, secretRepo.getCalled)
}

func TestCertificateManager_GetGatewayCertificateSecretData(t *testing.T) {
	caData := []byte("ca-data")
	secretWithCA := &apicorev1.Secret{
		Data: map[string][]byte{
			"ca.crt": caData,
		},
	}
	type fields struct {
		certClient certificate.CertificateRepository
		secretRepo certificate.SecretRepository
		config     certificate.Config
	}
	tests := []struct {
		name       string
		fields     fields
		want       *data.GatewaySecretData
		wantErr    assert.ErrorAssertionFunc
		getErrors  []error
		getSecrets []*apicorev1.Secret
	}{
		{
			name: "success returns GatewaySecretData",
			fields: fields{
				certClient: &certRepoStub{},
				secretRepo: &secretRepoStub{
					getSecrets: []*apicorev1.Secret{secretWithCA},
				},
				config: certificate.Config{
					GatewaySecretName: gatewaySecretName,
				},
			},
			want: &data.GatewaySecretData{
				CaCert: caData,
			},
			wantErr: assert.NoError,
		},
		{
			name: "error from secret client",
			fields: fields{
				certClient: &certRepoStub{},
				secretRepo: &secretRepoStub{
					getErrors: []error{assert.AnError},
				},
				config: certificate.Config{
					GatewaySecretName: gatewaySecretName,
				},
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cert := certificate.NewService(testCase.fields.certClient,
				testCase.fields.secretRepo,
				testCase.fields.config,
			)
			got, err := cert.GetGatewayCertificateSecretData(t.Context())
			if !testCase.wantErr(t, err) {
				return
			}
			if testCase.want != nil {
				assert.Equal(t, testCase.want.CaCert, got.CaCert)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func TestCertificateManager_GetSkrCertificateSecretData(t *testing.T) {
	tlsData := []byte("tls-data")
	keyData := []byte("key-data")
	secretWithCert := &apicorev1.Secret{
		Data: map[string][]byte{
			apicorev1.TLSCertKey:       tlsData,
			apicorev1.TLSPrivateKeyKey: keyData,
		},
	}
	type fields struct {
		certClient certificate.CertificateRepository
		secretRepo certificate.SecretRepository
		config     certificate.Config
	}
	tests := []struct {
		name       string
		fields     fields
		kymaName   string
		want       *data.CertificateSecretData
		wantErr    assert.ErrorAssertionFunc
		getErrors  []error
		getSecrets []*apicorev1.Secret
	}{
		{
			name: "success returns CertificateSecretData",
			fields: fields{
				certClient: &certRepoStub{},
				secretRepo: &secretRepoStub{
					getSecrets: []*apicorev1.Secret{secretWithCert},
				},
				config: certificate.Config{
					// CertificateNamespace: certNamespace,
				},
			},
			kymaName: kymaName,
			want: &data.CertificateSecretData{
				TlsCert: tlsData,
				TlsKey:  keyData,
			},
			wantErr: assert.NoError,
		},
		{
			name: "error from secret client",
			fields: fields{
				certClient: &certRepoStub{},
				secretRepo: &secretRepoStub{
					getErrors: []error{assert.AnError},
				},
				config: certificate.Config{
					// CertificateNamespace: certNamespace,
				},
			},
			kymaName: kymaName,
			want:     nil,
			wantErr:  assert.Error,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			cert := certificate.NewService(testCase.fields.certClient,
				testCase.fields.secretRepo,
				testCase.fields.config,
			)
			got, err := cert.GetSkrCertificateSecretData(t.Context(), testCase.kymaName)
			if !testCase.wantErr(t, err) {
				return
			}
			if testCase.want != nil {
				assert.Equal(t, testCase.want.TlsCert, got.TlsCert)
				assert.Equal(t, testCase.want.TlsKey, got.TlsKey)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

// Test stubs

type certRepoStub struct {
	createCalled         bool
	createErr            error
	createName           string
	createCommonName     string
	createDNSNames       []string
	deleteCalled         bool
	deleteErr            error
	deleteName           string
	getRenewalTimeCalled bool
	renewalTime          time.Time
	getRenewalTimeErr    error
	renewCalled          bool
	renewErr             error
	getValidityCalled    bool
	getValidityErr       error
	getValidityStart     time.Time
	getValidityEnd       time.Time
	receivedCertName     string
}

func (c *certRepoStub) Create(_ context.Context, name, commonName string,
	dnsNames []string,
) error {
	c.createCalled = true
	c.createName = name
	c.createCommonName = commonName
	c.createDNSNames = dnsNames
	return c.createErr
}

func (c *certRepoStub) Delete(_ context.Context, name string) error {
	c.deleteCalled = true
	c.deleteName = name
	return c.deleteErr
}

func (c *certRepoStub) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (c *certRepoStub) GetRenewalTime(_ context.Context, _ string) (time.Time, error) {
	c.getRenewalTimeCalled = true
	return c.renewalTime, c.getRenewalTimeErr
}

type secretRepoStub struct {
	getCalled          bool
	getErrors          []error
	getSecrets         []*apicorev1.Secret
	deleteCalled       bool
	deleteName         string
	deleteErr          error
	receivedSecretName string
}

func (c *secretRepoStub) Get(_ context.Context, secretName string) (*apicorev1.Secret, error) {
	c.getCalled = true
	c.receivedSecretName = secretName

	var secret *apicorev1.Secret
	if len(c.getSecrets) > 0 {
		secret = c.getSecrets[0]
		c.getSecrets = c.getSecrets[1:]
	}

	var err error
	if len(c.getErrors) > 0 {
		err = c.getErrors[0]
		c.getErrors = c.getErrors[1:]
	}

	return secret, err
}

func (c *secretRepoStub) Delete(_ context.Context, name string) error {
	c.deleteCalled = true
	c.deleteName = name
	return c.deleteErr
}
