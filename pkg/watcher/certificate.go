package watcher

import (
	"context"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	// private key will only be generated if one does not already exist in the target `spec.secretName`.
	privateKeyRotationPolicy = "Never"

	DomainAnnotation = v1beta2.SKRDomainAnnotation

	caCertKey        = "ca.crt"
	tlsCertKey       = "tls.crt"
	tlsPrivateKeyKey = "tls.key"
)

type SubjectAltName struct {
	DNSNames       []string
	IPAddresses    []string
	URIs           []string
	EmailAddresses []string
}

type CertificateManager struct {
	kcpClient           client.Client
	kyma                *v1beta2.Kyma
	caCertCache         *CertificateCache
	certificateName     string
	secretName          string
	istioNamespace      string
	remoteSyncNamespace string
	caCertName          string
	additionalDNSNames  []string
	labelSet            k8slabels.Set
}

type CertificateSecret struct {
	CACrt           string
	TLSCrt          string
	TLSKey          string
	ResourceVersion string
}

// NewCertificateManager returns a new CertificateManager, which can be used for creating a cert-manager Certificates.
func NewCertificateManager(kcpClient client.Client, kyma *v1beta2.Kyma,
	istioNamespace, remoteSyncNamespace, caCertName string, additionalDNSNames []string, caCertCache *CertificateCache,
) (*CertificateManager, error) {
	return &CertificateManager{
		kcpClient:           kcpClient,
		kyma:                kyma,
		certificateName:     ResolveTLSCertName(kyma.Name),
		secretName:          ResolveTLSCertName(kyma.Name),
		istioNamespace:      istioNamespace,
		remoteSyncNamespace: remoteSyncNamespace,
		caCertName:          caCertName,
		caCertCache:         caCertCache,
		additionalDNSNames:  additionalDNSNames,
		labelSet: k8slabels.Set{
			v1beta2.PurposeLabel: v1beta2.CertManager,
			v1beta2.ManagedBy:    v1beta2.OperatorName,
		},
	}, nil
}

// Create creates a cert-manager Certificate with a sufficient set of Subject-Alternative-Names.
func (c *CertificateManager) Create(ctx context.Context) error {
	// fetch Subject-Alternative-Name from KymaCR
	subjectAltNames, err := c.getSubjectAltNames()
	if err != nil {
		return fmt.Errorf("error get Subject Alternative Name from KymaCR: %w", err)
	}
	// create cert-manager CertificateCR
	err = c.createCertificate(ctx, subjectAltNames)
	if err != nil {
		return fmt.Errorf("error while creating certificate: %w", err)
	}
	return nil
}

// Remove removes the certificate including its certificate secret.
func (c *CertificateManager) Remove(ctx context.Context) error {
	err := c.RemoveCertificate(ctx)
	if err != nil {
		return err
	}

	return c.RemoveSecret(ctx)
}

func (c *CertificateManager) RemoveCertificate(ctx context.Context) error {
	certificate := &certmanagerv1.Certificate{}
	if err := c.kcpClient.Get(ctx, client.ObjectKey{
		Name:      c.certificateName,
		Namespace: c.istioNamespace,
	}, certificate); err != nil && !util.IsNotFound(err) {
		return fmt.Errorf("failed to get certificate: %w", err)
	}

	if err := c.kcpClient.Delete(ctx, certificate); err != nil {
		return fmt.Errorf("failed to delete certificate: %w", err)
	}

	return nil
}

func (c *CertificateManager) RemoveSecret(ctx context.Context) error {
	certSecret := &apicorev1.Secret{}
	if err := c.kcpClient.Get(ctx, client.ObjectKey{
		Name:      c.secretName,
		Namespace: c.istioNamespace,
	}, certSecret); err != nil && !util.IsNotFound(err) {
		return fmt.Errorf("failed to get certificate secret: %w", err)
	}

	err := c.kcpClient.Delete(ctx, certSecret)
	if err != nil {
		return fmt.Errorf("failed to delete certificate secret: %w", err)
	}
	return nil
}

func (c *CertificateManager) GetSecret(ctx context.Context) (*CertificateSecret, error) {
	secret := &apicorev1.Secret{}
	err := c.kcpClient.Get(ctx, client.ObjectKey{Name: c.secretName, Namespace: c.istioNamespace},
		secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret for certificate %s-%s: %w", c.secretName, c.istioNamespace, err)
	}
	certSecret := CertificateSecret{
		CACrt:           string(secret.Data[caCertKey]),
		TLSCrt:          string(secret.Data[tlsCertKey]),
		TLSKey:          string(secret.Data[tlsPrivateKeyKey]),
		ResourceVersion: secret.GetResourceVersion(),
	}
	return &certSecret, nil
}

func (c *CertificateManager) createCertificate(ctx context.Context, subjectAltName *SubjectAltName) error {
	// Default Duration 90 days
	// Default RenewBefore default 2/3 of Duration
	issuer, err := c.getIssuer(ctx)
	if err != nil {
		return fmt.Errorf("error getting issuer: %w", err)
	}

	cert := certmanagerv1.Certificate{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       certmanagerv1.CertificateKind,
			APIVersion: certmanagerv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      c.certificateName,
			Namespace: c.istioNamespace,
		},
		Spec: certmanagerv1.CertificateSpec{
			DNSNames:       subjectAltName.DNSNames,
			IPAddresses:    subjectAltName.IPAddresses,
			URIs:           subjectAltName.URIs,
			EmailAddresses: subjectAltName.EmailAddresses,
			SecretName:     c.secretName,
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: c.labelSet,
			},
			IssuerRef: certmanagermetav1.ObjectReference{
				Name: issuer.Name,
			},
			IsCA: false,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
			},
			PrivateKey: &certmanagerv1.CertificatePrivateKey{
				RotationPolicy: privateKeyRotationPolicy,
				Encoding:       certmanagerv1.PKCS1,
				Algorithm:      certmanagerv1.RSAKeyAlgorithm,
			},
		},
	}

	err = c.kcpClient.Patch(ctx, &cert, client.Apply, client.ForceOwnership, skrChartFieldOwner)
	if err != nil {
		return fmt.Errorf("failed to patch certificate: %w", err)
	}
	return nil
}

func (c *CertificateManager) getSubjectAltNames() (*SubjectAltName, error) {
	if domain, ok := c.kyma.Annotations[DomainAnnotation]; ok {
		if domain == "" {
			return nil, fmt.Errorf("Domain-Annotation of KymaCR %s is empty", c.kyma.Name) //nolint:goerr113
		}

		svcSuffix := []string{"svc.cluster.local", "svc"}
		dnsNames := []string{domain}

		for _, suffix := range svcSuffix {
			dnsNames = append(dnsNames, fmt.Sprintf("%s.%s.%s", SkrResourceName, c.remoteSyncNamespace, suffix))
		}

		dnsNames = append(dnsNames, c.additionalDNSNames...)

		return &SubjectAltName{
			DNSNames: dnsNames,
		}, nil
	}
	return nil, fmt.Errorf("kymaCR %s does not contain annotation '%s' with specified domain", //nolint:goerr113
		c.kyma.Name, DomainAnnotation)
}

func (c *CertificateManager) getIssuer(ctx context.Context) (*certmanagerv1.Issuer, error) {
	logger := logf.FromContext(ctx)
	issuerList := &certmanagerv1.IssuerList{}
	err := c.kcpClient.List(ctx, issuerList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(c.labelSet),
		Namespace:     c.istioNamespace,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list cert-manager issuer %w", err)
	}
	if len(issuerList.Items) == 0 {
		return nil, fmt.Errorf("no issuer found in Namespace `%s`", c.istioNamespace) //nolint:goerr113
	} else if len(issuerList.Items) > 1 {
		logger.Info("Found more than one issuer, will use by default first one in list",
			"issuer", issuerList.Items)
	}
	return &issuerList.Items[0], nil
}

func (c *CertificateManager) GetCertificateSecret(ctx context.Context) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	err := c.kcpClient.Get(ctx, client.ObjectKey{Name: c.secretName, Namespace: c.istioNamespace},
		secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret for certificate %s-%s: %w", c.secretName, c.istioNamespace, err)
	}

	return secret, nil
}

type CertificateNotReadyError struct{}

func (e *CertificateNotReadyError) Error() string {
	return "Certificate-Secret does not exist"
}

func (c *CertificateManager) GetCACertificate(ctx context.Context) (*certmanagerv1.Certificate, error) {
	cachedCert := c.caCertCache.GetCACertFromCache(c.caCertName)

	if cachedCert == nil || certificateRenewalTimePassed(cachedCert) {
		caCert := &certmanagerv1.Certificate{}
		if err := c.kcpClient.Get(ctx, client.ObjectKey{Namespace: c.istioNamespace, Name: c.caCertName},
			caCert); err != nil {
			return nil, fmt.Errorf("failed to get CA certificate %w", err)
		}
		c.caCertCache.SetCACertToCache(caCert)
		return caCert, nil
	}

	return cachedCert, nil
}

func certificateRenewalTimePassed(cert *certmanagerv1.Certificate) bool {
	return cert.Status.RenewalTime.Before(&(apimetav1.Time{Time: time.Now()}))
}
