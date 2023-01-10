package certmanager

import (
	"context"
	"fmt"

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
	// private key will only be generated if one does not already exist in the target `spec.secretName`
	privateKeyRotationPolicy = "Never"

	DomainAnnotation = "skr-domain"

	CertificateSuffix = "-watcher-certificate"
)

var (
	secretTemplateLabels = map[string]string{
		v1alpha1.PurposeLabel: v1alpha1.CertManager,
		v1alpha1.ManagedBy:    v1alpha1.OperatorName,
	}

	IssuerLabelSet = k8slabels.Set{"app.kubernetes.io/name": "lifecycle-manager"}
)

type SubjectAltName struct {
	DNSNames       []string
	IPAddresses    []string
	URIs           []string
	EmailAddresses []string
}

type certificate struct {
	ctx             context.Context
	namespace       string
	kcpClient       client.Client
	kyma            *v1alpha1.Kyma
	certificateName string
	secretName      string
}

func NewCertificate(ctx context.Context, kcpClient client.Client, kyma *v1alpha1.Kyma) (*certificate, error) {
	if ctx == nil || kcpClient == nil || kyma == nil {
		return nil, fmt.Errorf("could not create CertManager, context, client or Kyma must not be empty")
	}
	return &certificate{
		ctx:             ctx,
		kcpClient:       kcpClient,
		kyma:            kyma,
		certificateName: fmt.Sprintf("%s%s", kyma.Name, CertificateSuffix),
		secretName:      fmt.Sprintf("%s%s", kyma.Name, CertificateSuffix),
	}, nil
}

func (c *certificate) Create() error {
	// Check if Certificate exists
	exists, err := c.exists()
	if exists {
		return nil
	} else if err != nil {
		return err
	}
	// fetch Subject-Alternative-Name from KymaCR
	subjectAltNames, err := c.getSubjectAltNames()
	if err != nil {
		return err
	}
	// create cert-manager CertificateCR
	err = c.createCertificate(c.ctx, c.kyma.Namespace, subjectAltNames)
	if err != nil {
		return err
	}
	return nil
}

func (c *certificate) exists() (bool, error) {
	cert := v1.Certificate{}
	err := c.kcpClient.Get(c.ctx, types.NamespacedName{
		Namespace: c.kyma.Namespace,
		Name:      c.certificateName,
	}, &cert)
	if k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (c *certificate) createCertificate(
	ctx context.Context, certNamespace string,
	subjectAltName *SubjectAltName,
) error {
	//Duration Default 90 days
	//RenewBefore default 2/3 of Duration
	issuer, err := c.getIssuer()
	if err != nil {
		return err
	}

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
				Labels: secretTemplateLabels,
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

func (c *certificate) getSubjectAltNames() (*SubjectAltName, error) {
	if domain, ok := c.kyma.Annotations[DomainAnnotation]; ok {
		if domain == "" {
			return nil, fmt.Errorf("Domain-Annotation of KymaCR %s is empty", c.kyma.Name)
		}
		return &SubjectAltName{
			DNSNames: []string{domain},
		}, nil
	}
	return nil, fmt.Errorf("kymaCR %s does not contain annotation '%s' with specified domain",
		c.kyma.Name, DomainAnnotation)
}

// TODO double check, if we can use self-signed Issuer with `lifecycle-manager` label
func (c *certificate) getIssuer() (*v1.Issuer, error) {
	issuerList := &v1.IssuerList{}
	err := c.kcpClient.List(c.ctx, issuerList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(IssuerLabelSet),
		Namespace:     c.kyma.Namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("could not list cert-manager issuer %w", err)
	}
	if len(issuerList.Items) == 0 {
		return nil, fmt.Errorf("no issuer found")
	}
	return &issuerList.Items[0], nil
}
