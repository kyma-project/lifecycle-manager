package certificate

import (
	"context"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	certerror "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/errors"
)

// GetCacheObjects returns a list of objects that need to be cached for this client.
func GetCacheObjects() []client.Object {
	return []client.Object{
		&certmanagerv1.Certificate{},
	}
}

type Repository struct {
	kcpClient  client.Client
	issuerName string
	certConfig config.CertificateValues
}

func NewRepository(
	kcpClient client.Client,
	issuerName string,
	certConfig config.CertificateValues,
) (*Repository, error) {
	if certConfig.Namespace == "" {
		return nil, certerror.ErrCertRepoConfigNamespace
	}

	return &Repository{
		kcpClient,
		issuerName,
		certConfig,
	}, nil
}

func (r *Repository) Create(ctx context.Context, name, commonName string, dnsNames []string) error {
	cert := &certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: r.certConfig.Namespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			CommonName: commonName,
			Subject: &certmanagerv1.X509Subject{
				OrganizationalUnits: []string{certificate.DefaultOrganizationalUnit},
				Organizations:       []string{certificate.DefaultOrganization},
				Localities:          []string{certificate.DefaultLocality},
				Provinces:           []string{certificate.DefaultProvince},
				Countries:           []string{certificate.DefaultCountry},
			},
			Duration:    &apimetav1.Duration{Duration: r.certConfig.Duration},
			RenewBefore: &apimetav1.Duration{Duration: r.certConfig.RenewBefore},
			DNSNames:    dnsNames,
			SecretName:  name,
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: certificate.GetCertificateLabels(),
			},
			IssuerRef: certmanagermetav1.ObjectReference{
				Name: r.issuerName,
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
				Size:           r.certConfig.KeySize,
			},
		},
	}

	// Patch instead of Create + IgnoreAlreadyExists for cases where we change the config of certificates, e.g. duration
	err := r.kcpClient.Patch(ctx,
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

func (r *Repository) Delete(ctx context.Context, name string) error {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(r.certConfig.Namespace)

	if err := r.kcpClient.Delete(ctx, cert); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete certificate %s-%s: %w", name, r.certConfig.Namespace, err)
	}

	return nil
}

func (r *Repository) GetRenewalTime(ctx context.Context, name string) (time.Time, error) {
	cert, err := r.getCertificate(ctx, name)
	if err != nil {
		return time.Time{}, err
	}

	if cert.Status.RenewalTime == nil || cert.Status.RenewalTime.Time.IsZero() {
		return time.Time{}, certificate.ErrNoRenewalTime
	}

	return cert.Status.RenewalTime.Time, nil
}

func (r *Repository) GetValidity(ctx context.Context, name string) (time.Time, time.Time, error) {
	cert, err := r.getCertificate(ctx, name)
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

func (r *Repository) getCertificate(ctx context.Context, name string) (*certmanagerv1.Certificate, error) {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(r.certConfig.Namespace)

	if err := r.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
		return nil, fmt.Errorf("failed to get certificate %s-%s: %w", name, r.certConfig.Namespace, err)
	}

	return cert, nil
}
