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
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/skrwebhook/certificate/secret/data"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

var (
	ErrDomainAnnotationEmpty   = errors.New("domain annotation is empty")
	ErrDomainAnnotationMissing = errors.New("domain annotation is missing")
)

type CertRepository interface {
	Create(ctx context.Context, name, commonName string, dnsNames []string) error
	Delete(ctx context.Context, name string) error
	GetRenewalTime(ctx context.Context, name string) (time.Time, error)
}

type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
	Delete(ctx context.Context, name string) error
}

type Config struct {
	SkrServiceName     string
	SkrNamespace       string
	AdditionalDNSNames []string
	GatewaySecretName  string
	RenewBuffer        time.Duration
}

type SKRCertService struct {
	certRepo   CertRepository
	secretRepo SecretRepository
	config     Config
}

func NewSKRCertService(certRepo CertRepository, secretRepo SecretRepository, config Config) *SKRCertService {
	return &SKRCertService{
		certRepo:   certRepo,
		secretRepo: secretRepo,
		config:     config,
	}
}

// CreateSkrCertificate creates a Certificate for the SKR that is signed by the CA Root Certificate.
// It is used for mTLS connections from SKR to KCP.
func (c *SKRCertService) CreateSkrCertificate(ctx context.Context, kyma *v1beta2.Kyma) error {
	dnsNames, err := c.constructDNSNames(kyma)
	if err != nil {
		return fmt.Errorf("failed to construct DNS names: %w", err)
	}
	certName := constructSkrCertificateName(kyma.Name)
	commonName := kyma.GetRuntimeID()

	if err = c.certRepo.Create(ctx, certName, commonName, dnsNames); err != nil {
		return fmt.Errorf("failed to create SKR certificate: %w", err)
	}

	return nil
}

// DeleteSkrCertificate deletes the SKR certificate including its certificate secret.
func (c *SKRCertService) DeleteSkrCertificate(ctx context.Context, kymaName string) error {
	err := c.certRepo.Delete(ctx, constructSkrCertificateName(kymaName))
	if err != nil {
		return fmt.Errorf("failed to delete SKR certificate: %w", err)
	}

	err = c.secretRepo.Delete(ctx, constructSkrCertificateName(kymaName))
	if err != nil {
		return fmt.Errorf("failed to delete SKR certificate secret: %w", err)
	}

	return nil
}

// RenewSkrCertificate checks if the gateway certificate secret has been rotated. If so, it renews
// the SKR certificate by removing its certificate secret which will trigger a new certificate to be issued.
func (c *SKRCertService) RenewSkrCertificate(ctx context.Context, kymaName string) error {
	gatewaySecret, err := c.secretRepo.Get(ctx, c.config.GatewaySecretName)
	if err != nil {
		return fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	skrCertificateSecret, err := c.secretRepo.Get(ctx, constructSkrCertificateName(kymaName))
	if err != nil {
		return fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	if !skrSecretRequiresRenewal(gatewaySecret, skrCertificateSecret) {
		return nil
	}

	logf.FromContext(ctx).V(log.DebugLevel).Info("CA Certificate was rotated, removing certificate",
		"kyma", kymaName)

	if err = c.secretRepo.Delete(ctx, constructSkrCertificateName(kymaName)); err != nil {
		return fmt.Errorf("failed to delete SKR certificate secret: %w", err)
	}

	return nil
}

// IsSkrCertificateRenewalOverdue checks if the SKR certificate renewal is overdue.
func (c *SKRCertService) IsSkrCertificateRenewalOverdue(ctx context.Context, kymaName string) (bool, error) {
	renewalTime, err := c.certRepo.GetRenewalTime(ctx, constructSkrCertificateName(kymaName))
	if err != nil {
		return false, fmt.Errorf("failed to get SKR certificate renewal time: %w", err)
	}

	return time.Now().After(renewalTime.Add(c.config.RenewBuffer)), nil
}

// GetSkrCertificateSecret returns the SKR certificate secret.
// If the secret does not exist, it returns the ErrSkrCertificateNotReady error.
func (c *SKRCertService) GetSkrCertificateSecret(ctx context.Context, kymaName string) (*apicorev1.Secret, error) {
	skrCertSecret, err := c.secretRepo.Get(ctx, constructSkrCertificateName(kymaName))
	if err != nil {
		return nil, fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	return skrCertSecret, nil
}

func (c *SKRCertService) GetSkrCertificateSecretData(ctx context.Context,
	kymaName string,
) (*data.CertificateSecretData, error) {
	skrCertSecret, err := c.GetSkrCertificateSecret(ctx, kymaName)
	if err != nil {
		return nil, err
	}

	return data.NewCertificateSecretData(skrCertSecret)
}

// GetGatewayCertificateSecret returns the gateway certificate secret.
func (c *SKRCertService) GetGatewayCertificateSecret(ctx context.Context) (*apicorev1.Secret, error) {
	gatewayCertSecret, err := c.secretRepo.Get(ctx, c.config.GatewaySecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	return gatewayCertSecret, nil
}

func (c *SKRCertService) GetGatewayCertificateSecretData(ctx context.Context) (*data.GatewaySecretData, error) {
	gatewayCertSecret, err := c.GetGatewayCertificateSecret(ctx)
	if err != nil {
		return nil, err
	}

	return data.NewGatewaySecretData(gatewayCertSecret)
}

// renewal is required if the gateway certificate secret is newer than the SKR certificate secret.
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
func (c *SKRCertService) constructDNSNames(kyma *v1beta2.Kyma) ([]string, error) {
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

func constructSkrCertificateName(kymaName string) string {
	return kymaName + "-webhook-tls"
}
