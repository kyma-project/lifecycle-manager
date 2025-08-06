package gardener

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"

	gcertv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
)

var (
	ErrKeySizeOutOfRange                  = errors.New("KeySize is out of range for int32")
	ErrInputStringNotContainValidDates    = errors.New("input string does not contain valid dates")
	ErrCertificateStatusNotContainMessage = errors.New("certificate status does not contain message")
	dateRegex                             = regexp.MustCompile(`valid from (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? [+-]\d{4} UTC) to (\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? [+-]\d{4} UTC)`)
)

const regexMatchesCount = 3

// GetCacheObjects returns a list of objects that need to be cached for this client.
func GetCacheObjects() []client.Object {
	return []client.Object{
		&gcertv1alpha1.Certificate{},
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
	kcpClient       kcpClient
	issuerName      string
	issuerNamespace string
	config          certificate.CertificateConfig
}

func NewCertificateClient(kcpClient kcpClient,
	issuerName string,
	issuerNamespace string,
	config certificate.CertificateConfig,
) (*CertificateClient, error) {
	if config.KeySize > math.MaxInt32 || config.KeySize < math.MinInt32 {
		return nil, ErrKeySizeOutOfRange
	}

	return &CertificateClient{
		kcpClient,
		issuerName,
		issuerNamespace,
		config,
	}, nil
}

func (c *CertificateClient) Create(ctx context.Context,
	name string,
	namespace string,
	commonName string,
	dnsNames []string,
) error {
	//nolint:gosec // save as of the guard clause in constructor
	keySize := gcertv1alpha1.PrivateKeySize(int32(c.config.KeySize))
	rsaKeyAlgorithm := gcertv1alpha1.RSAKeyAlgorithm

	cert := &gcertv1alpha1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       gcertv1alpha1.CertificateKind,
			APIVersion: gcertv1alpha1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gcertv1alpha1.CertificateSpec{
			CommonName:   &commonName,
			Duration:     &apimetav1.Duration{Duration: c.config.Duration},
			DNSNames:     dnsNames,
			SecretName:   &name,
			SecretLabels: certificate.GetCertificateLabels(),
			IssuerRef: &gcertv1alpha1.IssuerRef{
				Name:      c.issuerName,
				Namespace: c.issuerNamespace,
			},
			PrivateKey: &gcertv1alpha1.CertificatePrivateKey{
				Algorithm: &rsaKeyAlgorithm,
				Size:      &keySize,
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
	cert := &gcertv1alpha1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(namespace)

	err := c.kcpClient.Delete(ctx, cert)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete certificate %s-%s: %w", name, namespace, err)
	}

	return nil
}

// GetRenewalTime returns the expiration date of the certificate minus the renewal time.
func (c *CertificateClient) GetRenewalTime(ctx context.Context,
	name string,
	namespace string,
) (time.Time, error) {
	cert := &gcertv1alpha1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(namespace)

	err := c.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get certificate %s-%s: %w", name, namespace, err)
	}

	if cert.Status.ExpirationDate == nil {
		return time.Time{}, fmt.Errorf("%w: no expiration date", certificate.ErrNoRenewalTime)
	}

	expirationDate, err := time.Parse(time.RFC3339, *cert.Status.ExpirationDate)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse certificate's expiration date '%s': %w",
			*cert.Status.ExpirationDate, err)
	}

	return expirationDate.Add(-c.config.RenewBefore), nil
}

func (c *CertificateClient) GetValidity(ctx context.Context,
	name string,
	namespace string,
) (time.Time, time.Time, error) {
	cert := &gcertv1alpha1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(namespace)

	err := c.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to get certificate %s-%s: %w", name, namespace, err)
	}

	if cert.Status.Message == nil {
		return time.Time{}, time.Time{}, ErrCertificateStatusNotContainMessage
	}

	notBefore, notAfter, err := parseValidity(*cert.Status.Message)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to extract validity: %w", err)
	}

	return notBefore, notAfter, nil
}

func parseValidity(input string) (time.Time, time.Time, error) {
	matches := dateRegex.FindStringSubmatch(input)
	if len(matches) != regexMatchesCount {
		return time.Time{}, time.Time{}, ErrInputStringNotContainValidDates
	}

	layout := "2006-01-02 15:04:05 -0700 MST"

	notBefore, err := time.Parse(layout, matches[1])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse notBefore date: %w", err)
	}

	notAfter, err := time.Parse(layout, matches[2])
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse notAfter date: %w", err)
	}

	return notBefore, notAfter, nil
}
