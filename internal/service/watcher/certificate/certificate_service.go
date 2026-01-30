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
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/name"
	"github.com/kyma-project/lifecycle-manager/internal/service/watcher/certificate/secret/data"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
)

var (
	ErrDomainAnnotationEmpty              = errors.New("domain annotation is empty")
	ErrDomainAnnotationMissing            = errors.New("domain annotation is missing")
	ErrGatewaySecretMissingLastModifiedAt = errors.New("gateway secret is missing lastModifiedAt annotation")
)

type CertificateRepository interface {
	Create(ctx context.Context, name, commonName string, dnsNames []string) error
	Delete(ctx context.Context, name string) error
	GetRenewalTime(ctx context.Context, name string) (time.Time, error)
	Renew(ctx context.Context, name string) error
	GetValidity(ctx context.Context, name string) (time.Time, time.Time, error)
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

type Service struct {
	certRepo   CertificateRepository
	secretRepo SecretRepository
	config     Config
}

func NewService(
	certRepo CertificateRepository,
	secretRepo SecretRepository,
	config Config,
) *Service {
	return &Service{
		certRepo:   certRepo,
		secretRepo: secretRepo,
		config:     config,
	}
}

// CreateSkrCertificate creates a Certificate for the SKR that is signed by the CA Root Certificate.
// It is used for mTLS connections from SKR to KCP.
func (c *Service) CreateSkrCertificate(ctx context.Context, kyma *v1beta2.Kyma) error {
	dnsNames, err := c.constructDNSNames(kyma)
	if err != nil {
		return fmt.Errorf("failed to construct DNS names: %w", err)
	}
	certName := name.SkrCertificate(kyma.Name)
	commonName := kyma.GetRuntimeID()

	if err = c.certRepo.Create(ctx, certName, commonName, dnsNames); err != nil {
		return fmt.Errorf("failed to create SKR certificate: %w", err)
	}

	return nil
}

// DeleteSkrCertificate deletes the SKR certificate including its certificate secret.
func (c *Service) DeleteSkrCertificate(ctx context.Context, kymaName string) error {
	err := c.certRepo.Delete(ctx, name.SkrCertificate(kymaName))
	if err != nil {
		return fmt.Errorf("failed to delete SKR certificate: %w", err)
	}

	err = c.secretRepo.Delete(ctx, name.SkrCertificate(kymaName))
	if err != nil {
		return fmt.Errorf("failed to delete SKR certificate secret: %w", err)
	}

	return nil
}

// RenewSkrCertificate checks if the SKR certificate needs to be renwed and triggers renewal if so.
func (c *Service) RenewSkrCertificate(ctx context.Context, kymaName string) error {
	renewalNeeded, err := c.skrCertificateNeedsRenewal(ctx, name.SkrCertificate(kymaName))
	if err != nil {
		return fmt.Errorf("failed to determine if SKR certificate needs renewal: %w", err)
	}

	if !renewalNeeded {
		return nil
	}

	logf.FromContext(ctx).
		V(log.InfoLevel).
		Info("SKR certificate needs renewal", "certificate", name.SkrCertificate(kymaName))

	if err = c.certRepo.Renew(ctx, name.SkrCertificate(kymaName)); err != nil {
		return fmt.Errorf("failed to renew SKR certificate: %w", err)
	}

	logf.FromContext(ctx).
		V(log.InfoLevel).
		Info("SKR certificate renewed", "certificate", name.SkrCertificate(kymaName))

	return nil
}

// IsSkrCertificateRenewalOverdue checks if the SKR certificate renewal is overdue.
func (c *Service) IsSkrCertificateRenewalOverdue(ctx context.Context, kymaName string) (bool, error) {
	renewalTime, err := c.certRepo.GetRenewalTime(ctx, name.SkrCertificate(kymaName))
	if err != nil {
		return false, fmt.Errorf("failed to get SKR certificate renewal time: %w", err)
	}

	return time.Now().After(renewalTime.Add(c.config.RenewBuffer)), nil
}

// GetSkrCertificateSecret returns the SKR certificate secret.
// If the secret does not exist, it returns the ErrSkrCertificateNotReady error.
func (c *Service) GetSkrCertificateSecret(ctx context.Context, kymaName string) (*apicorev1.Secret, error) {
	skrCertSecret, err := c.secretRepo.Get(ctx, name.SkrCertificate(kymaName))
	if err != nil {
		return nil, fmt.Errorf("failed to get SKR certificate secret: %w", err)
	}

	return skrCertSecret, nil
}

func (c *Service) GetSkrCertificateSecretData(ctx context.Context,
	kymaName string,
) (*data.CertificateSecretData, error) {
	skrCertSecret, err := c.GetSkrCertificateSecret(ctx, kymaName)
	if err != nil {
		return nil, err
	}

	return data.NewCertificateSecretData(skrCertSecret)
}

func (c *Service) GetGatewayCertificateSecretData(ctx context.Context) (*data.GatewaySecretData, error) {
	gatewayCertSecret, err := c.secretRepo.Get(ctx, c.config.GatewaySecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway certificate secret: %w", err)
	}

	return data.NewGatewaySecretData(gatewayCertSecret)
}

// DNS names are
//   - "skr-domain" annotation of the Kyma CR
//   - local K8s addresses for the SKR service
//   - additional DNS names from the config
func (c *Service) constructDNSNames(kyma *v1beta2.Kyma) ([]string, error) {
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

func (s *Service) skrCertificateNeedsRenewal(ctx context.Context, certName string) (bool, error) {
	certNotBefore, _, err := s.certRepo.GetValidity(ctx, certName)
	if err != nil {
		return false, fmt.Errorf("failed to determine SKR client certificate validity: %w", err)
	}

	gatewaySecretLastModifiedAt, err := s.getGatewaySecretLastModifiedAt(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to determine gateway secret lastModifiedAt: %w", err)
	}

	return certNotBefore.Before(gatewaySecretLastModifiedAt), nil
}

func (s *Service) getGatewaySecretLastModifiedAt(ctx context.Context) (time.Time, error) {
	gatewaySecret, err := s.secretRepo.Get(ctx, s.config.GatewaySecretName)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get gateway secret: %w", err)
	}

	lastModifiedAtValue, ok := gatewaySecret.Annotations[shared.LastModifiedAtAnnotation]
	if !ok {
		return time.Time{}, ErrGatewaySecretMissingLastModifiedAt
	}

	lastModifiedAt, err := time.Parse(time.RFC3339, lastModifiedAtValue)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse gateway secret lastModifiedAt annotation: %w", err)
	}

	return lastModifiedAt, nil
}
