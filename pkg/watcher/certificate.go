package watcher

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimachinerymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// private key will only be generated if one does not already exist in the target `spec.secretName`.
	privateKeyRotationPolicy = "Never"

	DomainAnnotation  = "skr-domain"
	CertificateSuffix = "-watcher-certificate"

	caCertKey        = "ca.crt"
	tlsCertKey       = "tls.crt"
	tlsPrivateKeyKey = "tls.key"
)

var LabelSet = k8slabels.Set{ //nolint:gochecknoglobals
	v1alpha1.PurposeLabel: v1alpha1.CertManager,
	v1alpha1.ManagedBy:    v1alpha1.OperatorName,
}

type SubjectAltName struct {
	DNSNames       []string
	IPAddresses    []string
	URIs           []string
	EmailAddresses []string
}

type Certificate struct {
	kcpClient       client.Client
	kyma            *v1alpha1.Kyma
	certificateName string
	secretName      string
	logger          logr.Logger
}

type CertificateSecret struct {
	CACrt           string
	TLSCrt          string
	TLSKey          string
	ResourceVersion string
}

// NewCertificate returns a new Certificate, which can be used for creating a cert-manager Certificate.
func NewCertificate(kcpClient client.Client, kyma *v1alpha1.Kyma) (*Certificate, error) {
	if kcpClient == nil || kyma == nil {
		return nil, fmt.Errorf("could not create CertManager, client or Kyma must not be empty") //nolint:goerr113
	}
	return &Certificate{
		kcpClient:       kcpClient,
		kyma:            kyma,
		certificateName: fmt.Sprintf("%s%s", kyma.Name, CertificateSuffix),
		secretName:      CreateSecretName(client.ObjectKeyFromObject(kyma)),
	}, nil
}

func CreateSecretName(kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf("%s%s", kymaObjKey.Name, CertificateSuffix)
}

// Create creates a cert-manager Certificate with a sufficient set of Subject-Alternative-Names.
func (c *Certificate) Create(ctx context.Context, config *SkrChartManagerConfig) error {
	c.logger = logf.FromContext(ctx)
	// Check if Certificate exists
	exists, err := c.exists(ctx)
	if exists {
		return nil
	} else if err != nil {
		return err
	}
	// fetch Subject-Alternative-Name from KymaCR
	subjectAltNames, err := c.getSubjectAltNames(config)
	if err != nil {
		c.logger.Error(err, "Error get Subject Alternative Name from KymaCR")
		return err
	}
	// create cert-manager CertificateCR
	err = c.createCertificate(ctx, c.kyma.Namespace, subjectAltNames)
	if err != nil {
		c.logger.Error(err, "Error while creating certificate")
		return err
	}
	c.logger.Info("Successfully created CA Root Certificate")
	return nil
}

// Remove removes the certificate including its certificate secret.
func (c *Certificate) Remove(ctx context.Context) error {
	certificate := &v1.Certificate{}
	if err := c.kcpClient.Get(ctx, client.ObjectKey{
		Name:      c.certificateName,
		Namespace: c.kyma.Namespace,
	}, certificate); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := c.kcpClient.Delete(ctx, certificate); err != nil {
		return err
	}
	certSecret := &corev1.Secret{}
	if err := c.kcpClient.Get(ctx, client.ObjectKey{
		Name:      c.secretName,
		Namespace: c.kyma.Namespace,
	}, certificate); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := c.kcpClient.Delete(ctx, certSecret); err != nil {
		return err
	}
	return nil
}

func (c *Certificate) GetSecret(ctx context.Context) (*CertificateSecret, error) {
	secret := &corev1.Secret{}
	namespace := c.kyma.ObjectMeta.Namespace
	name := c.secretName
	err := c.kcpClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace},
		secret)
	if err != nil {
		return nil, err
	}
	certSecret := CertificateSecret{
		CACrt:           string(secret.Data[caCertKey]),
		TLSCrt:          string(secret.Data[tlsCertKey]),
		TLSKey:          string(secret.Data[tlsPrivateKeyKey]),
		ResourceVersion: secret.GetResourceVersion(),
	}
	return &certSecret, nil
}

func (c *Certificate) exists(ctx context.Context) (bool, error) {
	cert := v1.Certificate{}
	err := c.kcpClient.Get(ctx, types.NamespacedName{
		Namespace: c.kyma.Namespace,
		Name:      c.certificateName,
	}, &cert)
	if k8serrors.IsNotFound(err) {
		c.logger.Info("Certificate does not exist", "Certificate", c.certificateName)
		return false, nil
	} else if err != nil {
		c.logger.Info("Could not fetch certificate from local Cluster")
		return false, err
	}
	return true, nil
}

func (c *Certificate) createCertificate(
	ctx context.Context, certNamespace string,
	subjectAltName *SubjectAltName,
) error {
	// Duration Default 90 days
	// RenewBefore default 2/3 of Duration
	issuer, err := c.getIssuer(ctx)
	if err != nil {
		c.logger.Error(err, "Error getting Issuer")
		return err
	}
	c.logger.Info("Issuer found", "issuer", issuer)

	cert := v1.Certificate{
		ObjectMeta: apimachinerymetav1.ObjectMeta{
			Name:      c.certificateName,
			Namespace: certNamespace,
		},
		Spec: v1.CertificateSpec{
			DNSNames:       subjectAltName.DNSNames,
			IPAddresses:    subjectAltName.IPAddresses,
			URIs:           subjectAltName.URIs,
			EmailAddresses: subjectAltName.EmailAddresses,
			SecretName:     c.secretName, // Name of the secret which stored certificate
			SecretTemplate: &v1.CertificateSecretTemplate{
				Labels: LabelSet,
			},
			IssuerRef: metav1.ObjectReference{
				Name: issuer.Name,
			},
			IsCA: false,
			Usages: []v1.KeyUsage{
				v1.UsageDigitalSignature,
				v1.UsageKeyEncipherment,
			},
			PrivateKey: &v1.CertificatePrivateKey{
				RotationPolicy: privateKeyRotationPolicy,
				Encoding:       v1.PKCS1,
				Algorithm:      v1.RSAKeyAlgorithm,
			},
		},
	}

	return c.kcpClient.Create(ctx, &cert)
}

func (c *Certificate) getSubjectAltNames(config *SkrChartManagerConfig) (*SubjectAltName, error) {
	if domain, ok := c.kyma.Annotations[DomainAnnotation]; ok {
		if domain == "" {
			return nil, fmt.Errorf("Domain-Annotation of KymaCR %s is empty", c.kyma.Name) //nolint:goerr113
		}

		resourceName := ResolveSKRChartResourceName(WebhookCfgAndDeploymentNameTpl,
			client.ObjectKeyFromObject(c.kyma))
		namespace := c.kyma.Namespace
		svcSuffix := []string{"svc.cluster.local", "svc"}
		dnsNames := []string{domain}

		for _, suffix := range svcSuffix {
			dnsNames = append(dnsNames, fmt.Sprintf("%s.%s.%s", resourceName, namespace, suffix))
		}

		if config.WatcherLocalTestingEnabled {
			dnsNames = append(dnsNames, []string{"localhost", "127.0.0.1", "host.k3d.internal"}...)
		}

		return &SubjectAltName{
			DNSNames: dnsNames,
		}, nil
	}
	return nil, fmt.Errorf("kymaCR %s does not contain annotation '%s' with specified domain", //nolint:goerr113
		c.kyma.Name, DomainAnnotation)
}

func (c *Certificate) getIssuer(ctx context.Context) (*v1.Issuer, error) {
	issuerList := &v1.IssuerList{}
	err := c.kcpClient.List(ctx, issuerList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(LabelSet),
		Namespace:     c.kyma.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list cert-manager issuer %w", err)
	}
	if len(issuerList.Items) == 0 {
		return nil, fmt.Errorf("no issuer found in Namespace `%s`", c.kyma.Namespace) //nolint:goerr113
	}
	return &issuerList.Items[0], nil
}

type CertificateNotReadyError struct{}

func (e *CertificateNotReadyError) Error() string {
	return "Certificate-Secret does not exist"
}
