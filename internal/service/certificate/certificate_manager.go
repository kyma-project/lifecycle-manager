package certificate

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

var (
	ErrDomainAnnotationEmpty   = errors.New("domain annotation is empty")
	ErrDomainAnnotationMissing = errors.New("domain annotation is missing")
)

type Certificate interface {
	Create(
		ctx context.Context,
		name string,
		namespace string,
		commonName string,
		dnsNames []string,
	) error
	Delete(ctx context.Context,
		name string,
		namespace string,
	) error
	GetRenewalTime(ctx context.Context,
		name string,
		namespace string,
	) (time.Time, error)
}

type Secret interface {
	Get(ctx context.Context,
		name string,
		namespace string,
	) (*apicorev1.Secret, error)
	Delete(ctx context.Context,
		name string,
		namespace string,
	) error
}

type CertificateManagerConfig struct {
	SkrServiceName       string
	SkrNamespace         string
	CertificateNamespace string
	AdditionalDNSNames   []string
	GatewaySecretName    string
	RenewBuffer          time.Duration
	// SkrCertificateNamingTemplate is the template for the SKR certificate name.
	// It should contain one %s placeholder for the Kyma name.
	SkrCertificateNamingTemplate string
}

type CertificateManager struct {
	certificate Certificate
	secret      Secret
	config      CertificateManagerConfig
}

func NewCertificateManager(certClient Certificate,
	secretClient Secret,
	config CertificateManagerConfig,
) *CertificateManager {
	return &CertificateManager{
		certificate: certClient,
		secret:      secretClient,
		config:      config,
	}
}

// CreateSkrCertificate creates a Certificate for the SKR.
// The Certificate is signed by the CA Root Certificate and is used for mTLS connection from SKR to KCP.
func (c *CertificateManager) CreateSkrCertificate(ctx context.Context, kyma *v1beta2.Kyma) error {
	dnsNames, err := c.constuctDNSNames(kyma)
	if err != nil {
		return fmt.Errorf("failed to construct DNS names: %w", err)
	}

	if err = c.certificate.Create(ctx,
		c.constructSkrCertificateName(kyma.Name),
		c.config.CertificateNamespace,
		kyma.GetRuntimeID(),
		dnsNames,
	); err != nil {
		return fmt.Errorf("failed to create SKR certificate: %w", err)
	}

	return nil
}

// DeleteSkrCertificate deletes the SKR certificate including its certificate secret.
func (c *CertificateManager) DeleteSkrCertificate(ctx context.Context, kymaName string) error {
	err := c.certificate.Delete(ctx, c.constructSkrCertificateName(kymaName), c.config.CertificateNamespace)
	if err != nil {
		return fmt.Errorf("failed to delete SKR certificate: %w", err)
	}

	err = c.secret.Delete(ctx, c.constructSkrCertificateName(kymaName), c.config.CertificateNamespace)
	if err != nil {
		return fmt.Errorf("failed to delete SKR certificate secret: %w", err)
	}

	return nil
}

// RenewSKRCertificate checks if the gateway certificate secret has been rotated. If so, it renews
// the SKR certificate by removing its certificate secret which will trigger a new certificate to be issued.
func (c *CertificateManager) RenewSkrCertificate(ctx context.Context, kymaName string) error {
	gatewaySecret, err := c.secret.Get(ctx, c.config.GatewaySecretName, c.config.CertificateNamespace)
	if err != nil {
		return fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	skrCertificateSecret, err := c.secret.Get(ctx, c.constructSkrCertificateName(kymaName), c.config.CertificateNamespace)
	if err != nil {
		return fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	if !skrSecretRequiresRenewal(gatewaySecret, skrCertificateSecret) {
		return nil
	}

	logf.FromContext(ctx).V(log.DebugLevel).Info("CA Certificate was rotated, removing certificate",
		"kyma", kymaName)

	if err = c.secret.Delete(ctx, c.constructSkrCertificateName(kymaName), c.config.CertificateNamespace); err != nil {
		return fmt.Errorf("failed to delete SKR certificate secret: %w", err)
	}

	return nil
}

// IsSkrCertificateRenewalOverdue checks if the SKR certificate renewal is overdue.
func (c *CertificateManager) IsSkrCertificateRenewalOverdue(ctx context.Context, kymaName string) (bool, error) {
	renewalTime, err := c.certificate.GetRenewalTime(ctx, c.constructSkrCertificateName(kymaName), c.config.CertificateNamespace)
	if err != nil {
		return false, fmt.Errorf("failed to get SKR certificate renewal time: %w", err)
	}

	return time.Now().After(renewalTime.Add(c.config.RenewBuffer)), nil
}

// GetSkrCertificateSecret returns the SKR certificate secret.
// If the secret does not exist, it returns the ErrSkrCertificateNotReady error.
func (c *CertificateManager) GetSkrCertificateSecret(ctx context.Context, kymaName string) (*apicorev1.Secret, error) {
	secret, err := c.secret.Get(ctx,
		c.constructSkrCertificateName(kymaName),
		c.config.CertificateNamespace,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	return secret, nil
}

// GetGatewayCertificateSecret returns the gateway certificate secret.
func (c *CertificateManager) GetGatewayCertificateSecret(ctx context.Context) (*apicorev1.Secret, error) {
	secret, err := c.secret.Get(ctx,
		c.config.GatewaySecretName,
		c.config.CertificateNamespace,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	return secret, nil
}

// renewal is required if the gateway certficiate secret is newer than the SKR certificate secret.
func skrSecretRequiresRenewal(gatewaySecret *apicorev1.Secret, skrSecret *apicorev1.Secret) bool {
	gwSecretLastModifiedAtValue, ok := gatewaySecret.Annotations[shared.LastModifiedAtAnnotation]
	// always renew if the annotation is not set
	if !ok {
		return true
	}

	gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue)
	// always renew if unable to parse
	if err != nil {
		return true
	}

	return skrSecret.CreationTimestamp.Time.Before(gwSecretLastModifiedAt)
}

// DNS names are
//   - "skr-domain" annotation of the Kyma CR
//   - local K8s addresses for the SKR service
//   - additional DNS names from the config
func (c *CertificateManager) constuctDNSNames(kyma *v1beta2.Kyma) ([]string, error) {
	serviceSuffixes := []string{
		"svc.cluster.local",
		"svc",
	}

	skrDomain, found := kyma.Annotations[shared.SkrDomainAnnotation]

	if !found {
		return nil, fmt.Errorf("%w (Kyma: %s)", ErrDomainAnnotationMissing, kyma.Name)
	}

	if skrDomain == "" {
		return nil, fmt.Errorf("%w (Kyma: %s)", ErrDomainAnnotationEmpty, kyma.Name)
	}

	dnsNames := []string{skrDomain}
	dnsNames = append(dnsNames, c.config.AdditionalDNSNames...)

	for _, suffix := range serviceSuffixes {
		dnsNames = append(dnsNames, fmt.Sprintf("%s.%s.%s", c.config.SkrServiceName, c.config.SkrNamespace, suffix))
	}

	return dnsNames, nil
}

func (c *CertificateManager) constructSkrCertificateName(kymaName string) string {
	return fmt.Sprintf(c.config.SkrCertificateNamingTemplate, kymaName)
}
