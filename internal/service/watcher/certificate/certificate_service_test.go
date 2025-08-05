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
	certRepo := &certRepositoryStub{}
	certService := certificate.NewService(&renewalServiceStub{}, certRepo, &secretRepoStub{}, certificate.Config{
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
	assert.True(t, certRepo.createCalled)
	assert.Equal(t, runtimeID, certRepo.createCommonName)
	assert.Equal(t, kymaName+expectedCertNameSuffix, certRepo.createName)
	assert.Contains(t, certRepo.createDNSNames, skrDomainName)
	assert.Contains(t, certRepo.createDNSNames, additionalDNSNames[0])
	assert.Contains(t, certRepo.createDNSNames, additionalDNSNames[1])
	assert.Contains(t, certRepo.createDNSNames,
		fmt.Sprintf("%s.%s.svc.cluster.local", skrServiceName, skrNamespace))
	assert.Contains(t, certRepo.createDNSNames, fmt.Sprintf("%s.%s.svc", skrServiceName, skrNamespace))
}

func TestCreateSkrCertificate_CertificateRepositoryReturnsError_ReturnsError(t *testing.T) {
	certRepo := &certRepositoryStub{
		createErr: assert.AnError,
	}
	certService := certificate.NewService(&renewalServiceStub{}, certRepo, &secretRepoStub{}, certificate.Config{
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
	assert.True(t, certRepo.createCalled)
}

func TestCreateSkrCertificate_ErrDomainAnnotationMissing_ReturnsError(t *testing.T) {
	certRepo := &certRepositoryStub{}
	certService := certificate.NewService(&renewalServiceStub{}, certRepo, &secretRepoStub{}, certificate.Config{
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
	assert.False(t, certRepo.createCalled)
}

func TestCreateSkrCertificate_ErrDomainAnnotationEmpty_ReturnsError(t *testing.T) {
	certRepo := &certRepositoryStub{}
	certService := certificate.NewService(&renewalServiceStub{}, certRepo, &secretRepoStub{}, certificate.Config{
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
	assert.False(t, certRepo.createCalled)
}

func Test_CertificateManager_DeleteSkrCertificate_Success(t *testing.T) {
	certRepo := &certRepositoryStub{}
	secretRepo := &secretRepoStub{}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, certRepo.deleteCalled)
	assert.Equal(t, kymaName+expectedCertNameSuffix, certRepo.deleteName)
	assert.True(t, secretRepo.deleteCalled)
	assert.Equal(t, kymaName+expectedCertNameSuffix, secretRepo.deleteName)
}

func Test_CertificateManager_DeleteSkrCertificate_Error_OnCertificate(t *testing.T) {
	certRepo := &certRepositoryStub{
		deleteErr: assert.AnError,
	}
	secretRepo := &secretRepoStub{}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate:")
	assert.True(t, certRepo.deleteCalled)
	assert.False(t, secretRepo.deleteCalled)
}

func Test_CertificateManager_DeleteSkrCertificate_Error_OnSecret(t *testing.T) {
	certRepo := &certRepositoryStub{}
	secretRepo := &secretRepoStub{
		deleteErr: assert.AnError,
	}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate secret:")
	assert.True(t, certRepo.deleteCalled)
	assert.True(t, secretRepo.deleteCalled)
}

func TestRenewSkrCertificate_WhenSecretsIndicateRenew_CallsRenewalServiceRenew(t *testing.T) {
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{ // gateway secret, modified now
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
			{ // skr secret, created a minute ago
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      kymaName + expectedCertNameSuffix,
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	renewalService := &renewalServiceStub{}
	manager := certificate.NewService(renewalService, &certRepositoryStub{}, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretRepo.getCalled)
	assert.False(t, secretRepo.deleteCalled)
	assert.Equal(t, 1, renewalService.calls)
	assert.Equal(t, kymaName+expectedCertNameSuffix, renewalService.lastCallArg)
}

func TestRenewSkrCertificate_WhenSecretsIndicateNoRenew_RenewalServiceRenewIsNotCalled(t *testing.T) {
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{ // gateway secret, modified a minute ago
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Add(-time.Minute).Format(time.RFC3339),
					},
				},
			},
			{ // skr secret, created now
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      kymaName + expectedCertNameSuffix,
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now(),
					},
				},
			},
		},
	}
	renewalService := &renewalServiceStub{}
	manager := certificate.NewService(renewalService, &certRepositoryStub{}, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, 0, renewalService.calls)
	assert.Empty(t, renewalService.lastCallArg)
}

func TestRenewSkrCertificate_GatewaySecretHasNoLastModified_CallsRenewalService(t *testing.T) {
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{ // gateway secret, no last modified
				ObjectMeta: apimetav1.ObjectMeta{
					Name:        gatewaySecretName,
					Namespace:   certNamespace,
					Annotations: map[string]string{},
				},
			},
			{ // skr secret, created a minute ago
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      kymaName + expectedCertNameSuffix,
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	renewalService := &renewalServiceStub{}
	manager := certificate.NewService(renewalService, &certRepositoryStub{}, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, 1, renewalService.calls)
	assert.Equal(t, kymaName+expectedCertNameSuffix, renewalService.lastCallArg)
}

func TestRenewSkrCertificate_WhenGatewaySecretHasInvalidLastModified_CallsRenewalServiceRenew(t *testing.T) {
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{ // gateway secret, no last modified
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: "not a time",
					},
				},
			},
			{ // skr secret, created a minute ago
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      kymaName + expectedCertNameSuffix,
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	renewalService := &renewalServiceStub{}
	manager := certificate.NewService(renewalService, &certRepositoryStub{}, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, 1, renewalService.calls)
	assert.Equal(t, kymaName+expectedCertNameSuffix, renewalService.lastCallArg)
}

func TestRenewSkrCertificate_SecretRepositoryGetReturnsError_ReturnsError(t *testing.T) {
	secretRepo := &secretRepoStub{
		getErrors: []error{assert.AnError},
	}
	renewalService := &renewalServiceStub{}
	manager := certificate.NewService(renewalService, &certRepositoryStub{}, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to get gateway certificate secret")
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, 0, renewalService.calls)
}

func TestRenewSkrCertificate_RenewalServiceReturnsError_ReturnsError(t *testing.T) {
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{ // gateway secret, modified now
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
			{ // skr secret, created a minute ago
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      kymaName + expectedCertNameSuffix,
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	renewalService := &renewalServiceStub{
		err: assert.AnError,
	}
	manager := certificate.NewService(renewalService, &certRepositoryStub{}, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.ErrorIs(t, err, assert.AnError)
	require.ErrorContains(t, err, "failed to delete SKR certificate secret")
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, 1, renewalService.calls)
	assert.Equal(t, kymaName+expectedCertNameSuffix, renewalService.lastCallArg)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue(t *testing.T) {
	certRepo := &certRepositoryStub{
		// renewal time is one second out of buffer
		renewalTime: time.Now().Add(-renewBuffer - time.Second),
	}
	secretRepo := &secretRepoStub{}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, overdue)
	assert.True(t, certRepo.getRenewalTimeCalled)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue_NotOverdue(t *testing.T) {
	certRepo := &certRepositoryStub{
		// renewal time is one second within buffer
		renewalTime: time.Now().Add(-renewBuffer + time.Second),
	}
	secretRepo := &secretRepoStub{}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	assert.False(t, overdue)
	assert.True(t, certRepo.getRenewalTimeCalled)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue_Error(t *testing.T) {
	certRepo := &certRepositoryStub{
		getRenewalTimeErr: assert.AnError,
	}
	secretRepo := &secretRepoStub{}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{
		RenewBuffer: renewBuffer,
	})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate renewal time:")
	assert.False(t, overdue)
	assert.True(t, certRepo.getRenewalTimeCalled)
}

func Test_CertificateManager_GetSkrCertificateSecret(t *testing.T) {
	certRepo := &certRepositoryStub{}
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
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, kymaName+expectedCertNameSuffix, result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func Test_CertificateManager_GetSkrCertificateSecret_Error(t *testing.T) {
	certRepo := &certRepositoryStub{}
	secretRepo := &secretRepoStub{
		getErrors: []error{assert.AnError},
	}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.True(t, secretRepo.getCalled)
}

func Test_CertificateManager_GetSkrCertificateSecret_NotFound(t *testing.T) {
	certRepo := &certRepositoryStub{}
	secretRepo := &secretRepoStub{
		getErrors: []error{
			apierrors.NewNotFound(apicorev1.Resource("secrets"), kymaName+expectedCertNameSuffix),
		},
	}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.Nil(t, result)
	assert.True(t, secretRepo.getCalled)
}

func Test_CertificateManager_GetGatewayCertificateSecret(t *testing.T) {
	certRepo := &certRepositoryStub{}
	secretRepo := &secretRepoStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
				},
			},
		},
	}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	result, err := manager.GetGatewayCertificateSecret(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretRepo.getCalled)
	assert.Equal(t, gatewaySecretName, result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func Test_CertificateManager_GetGatewayCertificateSecret_Error(t *testing.T) {
	certRepo := &certRepositoryStub{}
	secretRepo := &secretRepoStub{
		getErrors: []error{assert.AnError},
	}
	manager := certificate.NewService(&renewalServiceStub{}, certRepo, secretRepo, certificate.Config{
		GatewaySecretName: gatewaySecretName,
	})

	result, err := manager.GetGatewayCertificateSecret(t.Context())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get gateway certificate secret:")
	assert.True(t, secretRepo.getCalled)
}

// Test stubs

type certRepositoryStub struct {
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
}

func (c *certRepositoryStub) Create(_ context.Context, name, commonName string,
	dnsNames []string,
) error {
	c.createCalled = true
	c.createName = name
	c.createCommonName = commonName
	c.createDNSNames = dnsNames
	return c.createErr
}

func (c *certRepositoryStub) Delete(_ context.Context, name string) error {
	c.deleteCalled = true
	c.deleteName = name
	return c.deleteErr
}

func (c *certRepositoryStub) GetRenewalTime(_ context.Context, _ string) (time.Time, error) {
	c.getRenewalTimeCalled = true
	return c.renewalTime, c.getRenewalTimeErr
}

type secretRepoStub struct {
	getCalled    bool
	getErrors    []error
	getSecrets   []*apicorev1.Secret
	deleteCalled bool
	deleteName   string
	deleteErr    error
}

func (c *secretRepoStub) Get(_ context.Context, _ string) (*apicorev1.Secret, error) {
	c.getCalled = true

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
				certClient: &certRepositoryStub{},
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
				certClient: &certRepositoryStub{},
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
			c := certificate.NewService(&renewalServiceStub{}, testCase.fields.certClient, testCase.fields.secretRepo, testCase.fields.config)
			got, err := c.GetGatewayCertificateSecretData(t.Context())
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
			"tls.crt": tlsData,
			"tls.key": keyData,
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
				certClient: &certRepositoryStub{},
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
				certClient: &certRepositoryStub{},
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
			c := certificate.NewService(&renewalServiceStub{}, testCase.fields.certClient, testCase.fields.secretRepo, testCase.fields.config)
			got, err := c.GetSkrCertificateSecretData(t.Context(), testCase.kymaName)
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

type renewalServiceStub struct {
	calls       int
	lastCallArg string
	err         error
}

func (r *renewalServiceStub) Renew(_ context.Context, name string) error {
	r.calls++
	r.lastCallArg = name
	if r.err != nil {
		return r.err
	}

	return nil
}
