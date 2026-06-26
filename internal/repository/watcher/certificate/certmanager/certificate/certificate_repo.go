package certificate

import (
	"context"
	"fmt"
	"time"

	certmanagerapiutil "github.com/cert-manager/cert-manager/pkg/api/util"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	certmanagerapplyv1 "github.com/cert-manager/cert-manager/pkg/client/applyconfigurations/certmanager/v1"
	certmanagerapplymetav1 "github.com/cert-manager/cert-manager/pkg/client/applyconfigurations/meta/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate"
	"github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/config"
	certerror "github.com/kyma-project/lifecycle-manager/internal/repository/watcher/certificate/errors"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	renewalReason  = "ManuallyTriggered"
	renewalMessage = "Certificate re-issuance manually triggered"
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
	// Apply (SSA) instead of Create + IgnoreAlreadyExists for cases where we change the config of certificates, e.g. duration
	certApply := certmanagerapplyv1.Certificate(name, r.certConfig.Namespace).
		WithSpec(certmanagerapplyv1.CertificateSpec().
			WithCommonName(commonName).
			WithSubject(certmanagerapplyv1.X509Subject().
				WithOrganizationalUnits(certificate.DefaultOrganizationalUnit).
				WithOrganizations(certificate.DefaultOrganization).
				WithLocalities(certificate.DefaultLocality).
				WithProvinces(certificate.DefaultProvince).
				WithCountries(certificate.DefaultCountry),
			).
			WithDuration(apimetav1.Duration{Duration: r.certConfig.Duration}).
			WithRenewBefore(apimetav1.Duration{Duration: r.certConfig.RenewBefore}).
			WithDNSNames(dnsNames...).
			WithSecretName(name).
			WithSecretTemplate(certmanagerapplyv1.CertificateSecretTemplate().
				WithLabels(certificate.GetCertificateLabels()),
			).
			WithIssuerRef(certmanagerapplymetav1.IssuerReference().
				WithName(r.issuerName).
				WithKind(certmanagerv1.IssuerKind),
			).
			WithIsCA(false).
			WithUsages(
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			).
			WithPrivateKey(certmanagerapplyv1.CertificatePrivateKey().
				WithRotationPolicy(certmanagerv1.RotationPolicyAlways).
				WithEncoding(certmanagerv1.PKCS1).
				WithAlgorithm(certmanagerv1.RSAKeyAlgorithm).
				WithSize(r.certConfig.KeySize),
			),
		)

	if err := r.kcpClient.Apply(ctx, certApply, client.ForceOwnership, fieldowners.LifecycleManager); err != nil {
		return fmt.Errorf("failed to apply certificate: %w", err)
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

func (r *Repository) Exists(ctx context.Context, name string) (bool, error) {
	_, err := r.getCertificate(ctx, name)
	if err != nil {
		if util.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("failed to check existence of certificate %s-%s: %w", name, r.certConfig.Namespace,
				err)
		}
		return false, nil
	}
	return true, nil
}

func (r *Repository) Renew(ctx context.Context, name string) error {
	cert, err := r.getCertificate(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get certificate %s-%s: %w", name, r.certConfig.Namespace, err)
	}

	certmanagerapiutil.SetCertificateCondition(cert,
		cert.Generation,
		certmanagerv1.CertificateConditionIssuing,
		certmanagermetav1.ConditionTrue,
		renewalReason,
		renewalMessage)

	if err := r.kcpClient.Status().Update(ctx, cert); err != nil {
		return fmt.Errorf("failed to update certificate %s-%s status: %w", name, r.certConfig.Namespace, err)
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
