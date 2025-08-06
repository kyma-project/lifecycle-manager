package certmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
)

var (
	ErrNoNotBefore = errors.New("notBefore not found")
	ErrNoNotAfter  = errors.New("notAfter not found")
)

// GetCacheObjects returns a list of objects that need to be cached for this client.
func GetCacheObjects() []client.Object {
	return []client.Object{
		&certmanagerv1.Certificate{},
	}
}

type kcpClient interface {
	Get(ctx context.Context,
		key client.ObjectKey,
		obj client.Object,
		opts ...client.GetOption,
	) error
	Delete(ctx context.Context,
		obj client.Object,
		opts ...client.DeleteOption,
	) error
	Patch(ctx context.Context,
		obj client.Object,
		patch client.Patch,
		opts ...client.PatchOption,
	) error
}

type CertificateClient struct {
	kcpClient  kcpClient
	issuerName string
	config     certificate.CertificateConfig
}

func NewCertificateClient(kcpClient kcpClient,
	issuerName string,
	config certificate.CertificateConfig,
) *CertificateClient {
	return &CertificateClient{
		kcpClient,
		issuerName,
		config,
	}
}

func (c *CertificateClient) Create(ctx context.Context,
	name string,
	namespace string,
	commonName string,
	dnsNames []string,
) error {
	cert := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			CommonName:  commonName,
			Duration:    &apimetav1.Duration{Duration: c.config.Duration},
			RenewBefore: &apimetav1.Duration{Duration: c.config.RenewBefore},
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
				Size:           c.config.KeySize,
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

func (c *CertificateClient) Delete(ctx context.Context,
	name string,
	namespace string,
) error {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(namespace)

	err := c.kcpClient.Delete(ctx, cert)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete certificate %s-%s: %w", name, namespace, err)
	}

	return nil
}

func (c *CertificateClient) GetRenewalTime(ctx context.Context,
	name string,
	namespace string,
) (time.Time, error) {
	cert, err := c.getCertificate(ctx, name, namespace)
	if err != nil {
		return time.Time{}, err
	}

	if cert.Status.RenewalTime == nil || cert.Status.RenewalTime.Time.IsZero() {
		return time.Time{}, certificate.ErrNoRenewalTime
	}

	return cert.Status.RenewalTime.Time, nil
}

func (c *CertificateClient) GetValidity(ctx context.Context,
	name string,
	namespace string,
) (time.Time, time.Time, error) {
	cert, err := c.getCertificate(ctx, name, namespace)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if cert.Status.NotBefore == nil {
		return time.Time{}, time.Time{}, ErrNoNotBefore
	}

	if cert.Status.NotAfter == nil {
		return time.Time{}, time.Time{}, ErrNoNotAfter
	}

	return cert.Status.NotBefore.Time, cert.Status.NotAfter.Time, nil
}

func (c *CertificateClient) getCertificate(ctx context.Context,
	name string,
	namespace string,
) (*certmanagerv1.Certificate, error) {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(namespace)

	err := c.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert)
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate %s-%s: %w", name, namespace, err)
	}

	return cert, nil
}
