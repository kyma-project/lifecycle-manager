package certmanager

import (
	"context"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/config"
	certerror "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/skrwebhook/certificate/errors"
)

// GetCacheObjects returns a list of objects that need to be cached for this client.
func GetCacheObjects() []client.Object {
	return []client.Object{
		&certmanagerv1.Certificate{},
	}
}

type CertificateRepository struct {
	kcpClient  client.Client
	issuerName string
	certConfig config.CertificateValues
}

func NewCertificateRepository(kcpClient client.Client, issuerName string, certConfig config.CertificateValues) (*CertificateRepository, error) {
	if certConfig.Namespace == "" {
		return nil, certerror.ErrCertRepoConfigNamespace
	}

	return &CertificateRepository{
		kcpClient,
		issuerName,
		certConfig,
	}, nil
}

func (c *CertificateRepository) Create(ctx context.Context, name, commonName string, dnsNames []string) error {
	cert := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: c.certConfig.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			CommonName:  commonName,
			Duration:    &apimetav1.Duration{Duration: c.certConfig.Duration},
			RenewBefore: &apimetav1.Duration{Duration: c.certConfig.RenewBefore},
			DNSNames:    dnsNames,
			SecretName:  name,
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: certificate.GetCertificateLabels(),
			},
			IssuerRef: certmanagermetav1.ObjectReference{
				Name: c.issuerName,
				Kind: certmanagerv1.IssuerKind,
			},
			IsCA: false,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				RotationPolicy: certmanagerv1.RotationPolicyAlways,
				Encoding:       certmanagerv1.PKCS1,
				Algorithm:      certmanagerv1.RSAKeyAlgorithm,
				Size:           c.certConfig.KeySize,
			},
		},
	}

	// Patch instead of Create + IgnoreAlreadyExists for cases where we change the config of certificates, e.g. duration
	err := c.kcpClient.Patch(ctx,
		cert,
		client.Apply,
		client.ForceOwnership,
		fieldowners.LifecycleManager,
	)
	if err != nil {
		return fmt.Errorf("failed to patch certificate: %w", err)
	}

	return nil
}

func (c *CertificateRepository) Delete(ctx context.Context, name string) error {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(c.certConfig.Namespace)

	if err := c.kcpClient.Delete(ctx, cert); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete certificate %s-%s: %w", name, c.certConfig.Namespace, err)
	}

	return nil
}

func (c *CertificateRepository) GetRenewalTime(ctx context.Context, name string) (time.Time, error) {
	cert, err := c.getCertificate(ctx, name)
	if err != nil {
		return time.Time{}, err
	}

	if cert.Status.RenewalTime == nil || cert.Status.RenewalTime.Time.IsZero() {
		return time.Time{}, certificate.ErrNoRenewalTime
	}

	return cert.Status.RenewalTime.Time, nil
}

func (c *CertificateRepository) GetValidity(ctx context.Context, name string) (time.Time, time.Time, error) {
	cert, err := c.getCertificate(ctx, name)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if cert.Status.NotBefore == nil {
		return time.Time{}, time.Time{}, certerror.ErrNoNotBefore
	}

	if cert.Status.NotAfter == nil {
		return time.Time{}, time.Time{}, certerror.ErrNoNotAfter
	}

	return cert.Status.NotBefore.Time, cert.Status.NotAfter.Time, nil
}

func (c *CertificateRepository) getCertificate(ctx context.Context, name string) (*certmanagerv1.Certificate, error) {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(c.certConfig.Namespace)

	if err := c.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
		return nil, fmt.Errorf("failed to get certificate %s-%s: %w", name, c.certConfig.Namespace, err)
	}

	return cert, nil
}
