package watcher

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
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

type CertificateManager struct {
	kcpClient                  client.Client
	kyma                       *v1alpha1.Kyma
	certificateName            string
	secretName                 string
	istioNamespace             string
	watcherLocalTestingEnabled bool
}

type CertificateSecret struct {
	CACrt           string
	TLSCrt          string
	TLSKey          string
	ResourceVersion string
}

// NewCertificateManager returns a new CertificateManager, which can be used for creating a cert-manager Certificates.
func NewCertificateManager(kcpClient client.Client, kyma *v1alpha1.Kyma,
	istioNamespace string, localTesting bool,
) (*CertificateManager, error) {
	return &CertificateManager{
		kcpClient:                  kcpClient,
		kyma:                       kyma,
		certificateName:            fmt.Sprintf("%s%s", kyma.Name, CertificateSuffix),
		secretName:                 CreateSecretName(client.ObjectKeyFromObject(kyma)),
		istioNamespace:             istioNamespace,
		watcherLocalTestingEnabled: localTesting,
	}, nil
}

func CreateSecretName(kymaObjKey client.ObjectKey) string {
	return fmt.Sprintf("%s%s", kymaObjKey.Name, CertificateSuffix)
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
	certificate := &v1.Certificate{}
	if err := c.kcpClient.Get(ctx, client.ObjectKey{
		Name:      c.certificateName,
		Namespace: c.istioNamespace,
	}, certificate); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if err := c.kcpClient.Delete(ctx, certificate); err != nil {
		return err
	}
	certSecret := &corev1.Secret{}
	if err := c.kcpClient.Get(ctx, client.ObjectKey{
		Name:      c.secretName,
		Namespace: c.istioNamespace,
	}, certificate); err != nil && !k8serrors.IsNotFound(err) {
		return err
	}
	if err := c.kcpClient.Delete(ctx, certSecret); err != nil {
		return err
	}
	return nil
}

func (c *CertificateManager) GetSecret(ctx context.Context) (*CertificateSecret, error) {
	secret := &corev1.Secret{}
	err := c.kcpClient.Get(ctx, client.ObjectKey{Name: c.secretName, Namespace: c.istioNamespace},
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

// TODO PKI Remove after SSA
func (c *CertificateManager) exists(ctx context.Context) (bool, error) {
	cert := v1.Certificate{}
	err := c.kcpClient.Get(ctx, types.NamespacedName{
		Namespace: c.kyma.Namespace,
		Name:      c.certificateName,
	}, &cert)
	if k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("could not fetch certificate from local Cluster: %w", err)
	}
	return true, nil
}

func (c *CertificateManager) createCertificate(ctx context.Context, subjectAltName *SubjectAltName) error {
	// Default Duration 90 days
	// Default RenewBefore default 2/3 of Duration
	issuer, err := c.getIssuer(ctx)
	if err != nil {
		return fmt.Errorf("error getting issuer: %w", err)
	}

	cert := v1.Certificate{
		TypeMeta: apimachinerymetav1.TypeMeta{
			Kind:       v1.CertificateKind,
			APIVersion: v1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimachinerymetav1.ObjectMeta{
			Name:      c.certificateName,
			Namespace: c.istioNamespace,
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

	return c.kcpClient.Patch(ctx, &cert, client.Apply, client.ForceOwnership, skrChartFieldOwner)
}

func (c *CertificateManager) getSubjectAltNames() (*SubjectAltName, error) {
	if domain, ok := c.kyma.Annotations[DomainAnnotation]; ok {
		if domain == "" {
			return nil, fmt.Errorf("Domain-Annotation of KymaCR %s is empty", c.kyma.Name) //nolint:goerr113
		}

		// TODO PKI double check after rebasing
		resourceName := ResolveSKRChartResourceName(WebhookCfgAndDeploymentNameTpl, client.ObjectKeyFromObject(c.kyma))
		svcSuffix := []string{"svc.cluster.local", "svc"}
		dnsNames := []string{domain}

		for _, suffix := range svcSuffix {
			dnsNames = append(dnsNames, fmt.Sprintf("%s.%s.%s", resourceName, c.kyma.Namespace, suffix))
		}

		if c.watcherLocalTestingEnabled {
			dnsNames = append(dnsNames, []string{"localhost", "127.0.0.1", "host.k3d.internal"}...)
		}

		return &SubjectAltName{
			DNSNames: dnsNames,
		}, nil
	}
	return nil, fmt.Errorf("kymaCR %s does not contain annotation '%s' with specified domain", //nolint:goerr113
		c.kyma.Name, DomainAnnotation)
}

func (c *CertificateManager) getIssuer(ctx context.Context) (*v1.Issuer, error) {
	logger := log.FromContext(ctx)
	issuerList := &v1.IssuerList{}
	err := c.kcpClient.List(ctx, issuerList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(LabelSet),
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

type CertificateNotReadyError struct{}

func (e *CertificateNotReadyError) Error() string {
	return "Certificate-Secret does not exist"
}
