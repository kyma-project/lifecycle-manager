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
	certsvc "github.com/kyma-project/lifecycle-manager/internal/service/certificate"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
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
	certStub := &certificateStub{}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
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
	assert.True(t, certStub.createCalled)
	assert.Equal(t, runtimeID, certStub.createCommonName)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), certStub.createName)
	assert.Equal(t, certNamespace, certStub.createNamespace)
	assert.Contains(t, certStub.createDNSNames, skrDomainName)
	assert.Contains(t, certStub.createDNSNames, additionalDNSNames[0])
	assert.Contains(t, certStub.createDNSNames, additionalDNSNames[1])
	assert.Contains(t, certStub.createDNSNames, fmt.Sprintf("%s.%s.svc.cluster.local", skrServiceName, skrNamepsace))
	assert.Contains(t, certStub.createDNSNames, fmt.Sprintf("%s.%s.svc", skrServiceName, skrNamepsace))
}

func Test_CertificateManager_CreateSkrCertificate_Error(t *testing.T) {
	certStub := &certificateStub{
		createErr: assert.AnError,
	}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
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
	assert.True(t, certStub.createCalled)
}

func Test_CertificateManager_CreateSkrCertificate_ErrDomainAnnotationMissing(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
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
	require.ErrorIs(t, err, certsvc.ErrDomainAnnotationMissing)
	assert.Contains(t, err.Error(), "failed to construct DNS names")
	assert.False(t, certStub.createCalled)
}

func Test_CertificateManager_CreateSkrCertificate_ErrDomainAnnotationEmpty(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
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
	require.ErrorIs(t, err, certsvc.ErrDomainAnnotationEmpty)
	assert.Contains(t, err.Error(), "failed to construct DNS names")
	assert.False(t, certStub.createCalled)
}

func Test_CertificateManager_DeleteSkrCertificate_Success(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, certStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), certStub.deleteName)
	assert.Equal(t, certNamespace, certStub.deleteNamespace)
	assert.True(t, secretStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretStub.deleteName)
	assert.Equal(t, certNamespace, secretStub.deleteNamespace)
}

func Test_CertificateManager_DeleteSkrCertificate_Error_OnCertificate(t *testing.T) {
	certStub := &certificateStub{
		deleteErr: assert.AnError,
	}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate:")
	assert.True(t, certStub.deleteCalled)
	assert.False(t, secretStub.deleteCalled)
}

func Test_CertificateManager_DeleteSkrCertificate_Error_OnSecret(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
		deleteErr: assert.AnError,
	}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	err := manager.DeleteSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate secret:")
	assert.True(t, certStub.deleteCalled)
	assert.True(t, secretStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Renew(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretStub.getCalled)
	assert.True(t, secretStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretStub.deleteName)
	assert.Equal(t, certNamespace, secretStub.deleteNamespace)
}

func Test_CertificateManager_RenewSkrCertificate_NoRenew(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretStub.getCalled)
	assert.False(t, secretStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Renew_NoLastModified(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretStub.getCalled)
	assert.True(t, secretStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretStub.deleteName)
	assert.Equal(t, certNamespace, secretStub.deleteNamespace)
}

func Test_CertificateManager_RenewSkrCertificate_Renew_InvalidLastModified(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, secretStub.getCalled)
	assert.True(t, secretStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretStub.deleteName)
	assert.Equal(t, certNamespace, secretStub.deleteNamespace)
}

func Test_CertificateManager_RenewSkrCertificate_Error_GatewaySecret(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get gateway certificate secret:")
	assert.True(t, secretStub.getCalled)
	assert.False(t, secretStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Error_SkrSecret(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.True(t, secretStub.getCalled)
	assert.False(t, secretStub.deleteCalled)
}

func Test_CertificateManager_RenewSkrCertificate_Error_Delete(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
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
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace:         certNamespace,
			GatewaySecretName:            gatewaySecretName,
			SkrCertificateNamingTemplate: certNamingTemplate,
		})

	err := manager.RenewSkrCertificate(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete SKR certificate secret:")
	assert.True(t, secretStub.getCalled)
	assert.True(t, secretStub.deleteCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), secretStub.deleteName)
	assert.Equal(t, certNamespace, secretStub.deleteNamespace)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue(t *testing.T) {
	certStub := &certificateStub{
		// renewal time is one second out of buffer
		renewalTime: time.Now().Add(-renewBuffer - time.Second),
	}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			RenewBuffer:          renewBuffer,
		})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	assert.True(t, overdue)
	assert.True(t, certStub.getRenewalTimeCalled)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue_NotOverdue(t *testing.T) {
	certStub := &certificateStub{
		// renewal time is one second within buffer
		renewalTime: time.Now().Add(-renewBuffer + time.Second),
	}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			RenewBuffer:          renewBuffer,
		})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.NoError(t, err)
	assert.False(t, overdue)
	assert.True(t, certStub.getRenewalTimeCalled)
}

func Test_CertificateManager_IsSkrCertificateRenewalOverdue_Error(t *testing.T) {
	certStub := &certificateStub{
		getRenewalTimeErr: assert.AnError,
	}
	secretStub := &secretStub{}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			RenewBuffer:          renewBuffer,
		})

	overdue, err := manager.IsSkrCertificateRenewalOverdue(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate renewal time:")
	assert.False(t, overdue)
	assert.True(t, certStub.getRenewalTimeCalled)
}

func Test_CertificateManager_GetSkrCertificateSecret(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      fmt.Sprintf(certNamingTemplate, kymaName),
					Namespace: certNamespace,
				},
			},
		},
	}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretStub.getCalled)
	assert.Equal(t, fmt.Sprintf(certNamingTemplate, kymaName), result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func Test_CertificateManager_GetSkrCertificateSecret_Error(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
		getErrors: []error{assert.AnError},
	}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.True(t, secretStub.getCalled)
}

func Test_CertificateManager_GetSkrCertificateSecret_NotFound(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
		getErrors: []error{apierrors.NewNotFound(apicorev1.Resource("secrets"), fmt.Sprintf(certNamingTemplate, kymaName))},
	}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
		})

	result, err := manager.GetSkrCertificateSecret(t.Context(), kymaName)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get SKR certificate secret:")
	assert.Nil(t, result)
	assert.True(t, secretStub.getCalled)
}

func Test_CertificateManager_GetGatewayCertificateSecret(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
		getSecrets: []*apicorev1.Secret{
			{
				ObjectMeta: apimetav1.ObjectMeta{
					Name:      gatewaySecretName,
					Namespace: certNamespace,
				},
			},
		},
	}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			GatewaySecretName:    gatewaySecretName,
		})

	result, err := manager.GetGatewayCertificateSecret(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, secretStub.getCalled)
	assert.Equal(t, gatewaySecretName, result.Name)
	assert.Equal(t, certNamespace, result.Namespace)
}

func Test_CertificateManager_GetGatewayCertificateSecret_Error(t *testing.T) {
	certStub := &certificateStub{}
	secretStub := &secretStub{
		getErrors: []error{assert.AnError},
	}
	manager := certsvc.NewCertificateManager(certStub,
		secretStub,
		certsvc.CertificateManagerConfig{
			CertificateNamespace: certNamespace,
			GatewaySecretName:    gatewaySecretName,
		})

	result, err := manager.GetGatewayCertificateSecret(t.Context())

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get gateway certificate secret:")
	assert.True(t, secretStub.getCalled)
}

// Test stubs

type certificateStub struct {
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

func (c *certificateStub) Create(ctx context.Context, name, namespace, commonName string, dnsNames []string) error {
	c.createCalled = true
	c.createName = name
	c.createNamespace = namespace
	c.createCommonName = commonName
	c.createDNSNames = dnsNames
	return c.createErr
}

func (c *certificateStub) Delete(ctx context.Context, name, namespace string) error {
	c.deleteCalled = true
	c.deleteName = name
	c.deleteNamespace = namespace
	return c.deleteErr
}

func (c *certificateStub) GetRenewalTime(ctx context.Context, name, namespace string) (time.Time, error) {
	c.getRenewalTimeCalled = true
	return c.renewalTime, c.getRenewalTimeErr
}

type secretStub struct {
	getCalled       bool
	getErrors       []error
	getSecrets      []*apicorev1.Secret
	deleteCalled    bool
	deleteName      string
	deleteNamespace string
	deleteErr       error
}

func (c *secretStub) Get(ctx context.Context, name, namespace string) (*apicorev1.Secret, error) {
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

func (c *secretStub) Delete(ctx context.Context, name, namespace string) error {
	c.deleteCalled = true
	c.deleteName = name
	c.deleteNamespace = namespace
	return c.deleteErr
}
