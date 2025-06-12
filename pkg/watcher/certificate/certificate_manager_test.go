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
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate/secret"
)

var (
	certNamingTemplate = "%s-something"

	kymaName           = random.Name()
	certNamespace      = random.Name()
	gatewaySecretName  = random.Name()
	skrServiceName     = random.Name()
	skrNamepsace       = random.Name()
	additionalDNSNames = []string{
		random.Name(),
		random.Name(),
	}
	skrDomainName = random.Name()
	runtimeID     = random.Name()

	renewBuffer = 10 * time.Minute
)

func Test_CertificateManager_CreateSkrCertificate_Success(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			AdditionalDNSNames:           additionalDNSNames,
			SkrCertificateNamingTemplate: certNamingTemplate,
			SkrServiceName:               skrServiceName,
			SkrNamespace:                 skrNamepsace,
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

	err := manager.CreateSkrCertificate(t.Context(), kyma)

	require.NoError(t, err)
	assert.True(t, certClientStub.createCalled)
	assert.Equal(t, runtimeID, certClientStub.createCommonName)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), certClientStub.createName)
	assert.Equal(t, certNamespace, certClientStub.createNamespace)
	assert.Contains(t, certClientStub.createDNSNames, skrDomainName)
	assert.Contains(t, certClientStub.createDNSNames, additionalDNSNames[0])
	assert.Contains(t, certClientStub.createDNSNames, additionalDNSNames[1])
	assert.Contains(t, certClientStub.createDNSNames,
		fmt.Sprintf("%s.%s.svc.cluster.local", skrServiceName, skrNamepsace))
	assert.Contains(t, certClientStub.createDNSNames, fmt.Sprintf("%s.%s.svc", skrServiceName, skrNamepsace))
}

func Test_CertificateManager_CreateSkrCertificate_Error(t *testing.T) {
	certClientStub := &certificateClientStub{
		createErr: assert.AnError,
	}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			AdditionalDNSNames:   additionalDNSNames,
		})

	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
			Annotations: map[string]string{
				shared.SkrDomainAnnotation: skrDomainName,
			},
		},
	}

	err := manager.CreateSkrCertificate(t.Context(), kyma)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create SKR certificate")
	assert.True(t, certClientStub.createCalled)
}

func Test_CertificateManager_CreateSkrCertificate_ErrDomainAnnotationMissing(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			AdditionalDNSNames:   additionalDNSNames,
		})

	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
		},
	}

	err := manager.CreateSkrCertificate(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, certificate.ErrDomainAnnotationMissing)
	assert.Contains(t, err.Error(), "failed to construct DNS names")
	assert.False(t, certClientStub.createCalled)
}

func Test_CertificateManager_CreateSkrCertificate_ErrDomainAnnotationEmpty(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			AdditionalDNSNames:   additionalDNSNames,
		})

	kyma := &v1beta2.Kyma{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: kymaName,
			Annotations: map[string]string{
				shared.SkrDomainAnnotation: "",
			},
		},
	}

	err := manager.CreateSkrCertificate(t.Context(), kyma)

	require.Error(t, err)
	require.ErrorIs(t, err, certificate.ErrDomainAnnotationEmpty)
	assert.Contains(t, err.Error(), "failed to construct DNS names")
	assert.False(t, certClientStub.createCalled)
}

func Test_CertificateManager_DeleteSkrCertificate_Success(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, certClientStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), certClientStub.deleteName)
	assert.Equal(t, certNamespace, certClientStub.deleteNamespace)
	assert.True(t, secretClientStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretClientStub.deleteName)
	assert.Equal(t, certNamespace, secretClientStub.deleteNamespace)
}

func Test_CertificateManager_DeleteSkrCertificate_Error_OnCertificate(t *testing.T) {
	certClientStub := &certificateClientStub{
		deleteErr: assert.AnError,
	}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate:")
	assert.True(t, certClientStub.deleteCalled)
	assert.False(t, secretClientStub.deleteCalled)
}

func Test_CertificateManager_DeleteSkrCertificate_Error_OnSecret(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		deleteErr: assert.AnError,
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate secret:")
	assert.True(t, certClientStub.deleteCalled)
	assert.True(t, secretClientStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Renew(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// gateway secret, modified now
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
			// skr secret, created a minute ago
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretClientStub.getCalled)
	assert.True(t, secretClientStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretClientStub.deleteName)
	assert.Equal(t, certNamespace, secretClientStub.deleteNamespace)
}

func Test_CertificateManager_RenewSkrCertificate_NoRenew(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// gateway secret, modified a minute ago
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Add(-time.Minute).Format(time.RFC3339),
					},
				},
			},
			// skr secret, created now
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now(),
					},
				},
			},
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretClientStub.getCalled)
	assert.False(t, secretClientStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Renew_NoLastModified(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// gateway secret, no last modified
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:        gatewaySecretName,
					Namespace:   certNamespace,
					Annotations: map[string]string{},
				},
			},
			// skr secret, created a minute ago
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretClientStub.getCalled)
	assert.True(t, secretClientStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretClientStub.deleteName)
	assert.Equal(t, certNamespace, secretClientStub.deleteNamespace)
}

func Test_CertificateManager_RenewSkrCertificate_Renew_InvalidLastModified(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// gateway secret, no last modified
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: "not a time",
					},
				},
			},
			// skr secret, created a minute ago
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretClientStub.getCalled)
	assert.True(t, secretClientStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretClientStub.deleteName)
	assert.Equal(t, certNamespace, secretClientStub.deleteNamespace)
}

func Test_CertificateManager_RenewSkrCertificate_Error_GatewaySecret(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// no gateway secret
			nil,
			// skr secret, created a minute ago
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
		getErrors: []error{assert.AnError},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get gateway certificate secret:")
	assert.True(t, secretClientStub.getCalled)
	assert.False(t, secretClientStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Error_SkrSecret(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// gateway secret, modified now
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
			// no skr secret
			nil,
		},
		getErrors: []error{nil, assert.AnError},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.True(t, secretClientStub.getCalled)
	assert.False(t, secretClientStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Error_Delete(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			// gateway secret, modified now
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
					Annotations: map[string]string{
						shared.LastModifiedAtAnnotation: time.Now().Format(time.RFC3339),
					},
				},
			},
			// skr secret, created a minute ago
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
					CreationTimestamp: apimetav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			},
		},
		deleteErr: assert.AnError,
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate secret:")
	assert.True(t, secretClientStub.getCalled)
	assert.True(t, secretClientStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretClientStub.deleteName)
	assert.Equal(t, certNamespace, secretClientStub.deleteNamespace)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue(t *testing.T) {
	certClientStub := &certificateClientStub{
		// renewal time is one second out of buffer
		renewalTime: time.Now().Add(-renewBuffer - time.Second),
	}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			RenewBuffer:          renewBuffer,
		})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, overdue)
	assert.True(t, certClientStub.getRenewalTimeCalled)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue_NotOverdue(t *testing.T) {
	certClientStub := &certificateClientStub{
		// renewal time is one second within buffer
		renewalTime: time.Now().Add(-renewBuffer + time.Second),
	}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			RenewBuffer:          renewBuffer,
		})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	assert.False(t, overdue)
	assert.True(t, certClientStub.getRenewalTimeCalled)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue_Error(t *testing.T) {
	certClientStub := &certificateClientStub{
		getRenewalTimeErr: assert.AnError,
	}
	secretClientStub := &certificateSecretClientStub{}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			RenewBuffer:          renewBuffer,
		})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate renewal time:")
	assert.False(t, overdue)
	assert.True(t, certClientStub.getRenewalTimeCalled)
}

func Test_CertificateManager_GetSkrCertificateSecret(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
				},
			},
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretClientStub.getCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func Test_CertificateManager_GetSkrCertificateSecret_Error(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getErrors: []error{assert.AnError},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.True(t, secretClientStub.getCalled)
}

func Test_CertificateManager_GetSkrCertificateSecret_NotFound(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getErrors: []error{
			apierrors.NewNotFound(apicorev1.Resource("secrets"), fmt.Sprintf(certNamingTemplate, kymaName)),
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.Nil(t, result)
	assert.True(t, secretClientStub.getCalled)
}

func Test_CertificateManager_GetGatewayCertificateSecret(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
				},
			},
		},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			GatewaySecretName:    gatewaySecretName,
		})

	result, err := manager.GetGatewayCertificateSecret(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretClientStub.getCalled)
	assert.Equal(t, gatewaySecretName, result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func Test_CertificateManager_GetGatewayCertificateSecret_Error(t *testing.T) {
	certClientStub := &certificateClientStub{}
	secretClientStub := &certificateSecretClientStub{
		getErrors: []error{assert.AnError},
	}
	manager := certificate.NewCertificateManager(certClientStub,
		secretClientStub,
		certificate.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			GatewaySecretName:    gatewaySecretName,
		})

	result, err := manager.GetGatewayCertificateSecret(t.Context())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get gateway certificate secret:")
	assert.True(t, secretClientStub.getCalled)
}

// Test stubs

type certificateClientStub struct {
	createCalled         bool
	createErr            error
	createName           string
	createNamespace      string
	createCommonName     string
	createDNSNames       []string
	deleteCalled         bool
	deleteErr            error
	deleteName           string
	deleteNamespace      string
	getRenewalTimeCalled bool
	renewalTime          time.Time
	getRenewalTimeErr    error
}

func (c *certificateClientStub) Create(ctx context.Context, name, namespace, commonName string,
	dnsNames []string,
) error {
	c.createCalled = true
	c.createName = name
	c.createNamespace = namespace
	c.createCommonName = commonName
	c.createDNSNames = dnsNames
	return c.createErr
}

func (c *certificateClientStub) Delete(ctx context.Context, name, namespace string) error {
	c.deleteCalled = true
	c.deleteName = name
	c.deleteNamespace = namespace
	return c.deleteErr
}

func (c *certificateClientStub) GetRenewalTime(ctx context.Context, name, namespace string) (time.Time, error) {
	c.getRenewalTimeCalled = true
	return c.renewalTime, c.getRenewalTimeErr
}

type certificateSecretClientStub struct {
	getCalled       bool
	getErrors       []error
	getSecrets      []*apicorev1.Secret
	deleteCalled    bool
	deleteName      string
	deleteNamespace string
	deleteErr       error
}

func (c *certificateSecretClientStub) Get(ctx context.Context, name, namespace string) (*apicorev1.Secret, error) {
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

func (c *certificateSecretClientStub) Delete(ctx context.Context, name, namespace string) error {
	c.deleteCalled = true
	c.deleteName = name
	c.deleteNamespace = namespace
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
		certClient   certificate.CertificateClient
		secretClient certificate.CertificateSecretClient
		config       certificate.CertificateManagerConfig
	}
	tests := []struct {
		name       string
		fields     fields
		want       *secret.GatewaySecretData
		wantErr    assert.ErrorAssertionFunc
		getErrors  []error
		getSecrets []*apicorev1.Secret
	}{
		{
			name: "success returns GatewaySecretData",
			fields: fields{
				certClient: &certificateClientStub{},
				secretClient: &certificateSecretClientStub{
					getSecrets: []*apicorev1.Secret{secretWithCA},
				},
				config: certificate.CertificateManagerConfig{
					CertificateNamespace: certNamespace,
					GatewaySecretName:    gatewaySecretName,
				},
			},
			want: &secret.GatewaySecretData{
				CaCert: caData,
			},
			wantErr: assert.NoError,
		},
		{
			name: "error from secret client",
			fields: fields{
				certClient: &certificateClientStub{},
				secretClient: &certificateSecretClientStub{
					getErrors: []error{assert.AnError},
				},
				config: certificate.CertificateManagerConfig{
					CertificateNamespace: certNamespace,
					GatewaySecretName:    gatewaySecretName,
				},
			},
			want:    nil,
			wantErr: assert.Error,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			c := certificate.NewCertificateManager(testCase.fields.certClient,
				testCase.fields.secretClient,
				testCase.fields.config)
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
		certClient   certificate.CertificateClient
		secretClient certificate.CertificateSecretClient
		config       certificate.CertificateManagerConfig
	}
	tests := []struct {
		name       string
		fields     fields
		kymaName   string
		want       *secret.CertificateSecretData
		wantErr    assert.ErrorAssertionFunc
		getErrors  []error
		getSecrets []*apicorev1.Secret
	}{
		{
			name: "success returns CertificateSecretData",
			fields: fields{
				certClient: &certificateClientStub{},
				secretClient: &certificateSecretClientStub{
					getSecrets: []*apicorev1.Secret{secretWithCert},
				},
				config: certificate.CertificateManagerConfig{
					CertificateNamespace: certNamespace,
				},
			},
			kymaName: kymaName,
			want: &secret.CertificateSecretData{
				TlsCert: tlsData,
				TlsKey:  keyData,
			},
			wantErr: assert.NoError,
		},
		{
			name: "error from secret client",
			fields: fields{
				certClient: &certificateClientStub{},
				secretClient: &certificateSecretClientStub{
					getErrors: []error{assert.AnError},
				},
				config: certificate.CertificateManagerConfig{
					CertificateNamespace: certNamespace,
				},
			},
			kymaName: kymaName,
			want:     nil,
			wantErr:  assert.Error,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			c := certificate.NewCertificateManager(
				testCase.fields.certClient,
				testCase.fields.secretClient,
				testCase.fields.config)
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
