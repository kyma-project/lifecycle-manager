package certificates

import (
	"context"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"

	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	metav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimachinerymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	apimachinerywait "k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// private key will only be generated if one does not already exist in the target `spec.secretName`
	privateKeyRotationPolicy = "Never"
	privateKeyEncoding       = "PKCS2"
	privateKeyAlgorithm      = "ed25519"

	domainAnnotation = "skr-domain"
)

var (
	secretLabels = map[string]string{
		v1alpha1.PurposeLabel: v1alpha1.CertManager,
		v1alpha1.ManagedBy:    v1alpha1.OperatorName,
	}
)

type SubjectAltName struct {
	DNSNames       []string
	IPAddresses    []string
	URIs           []string
	EmailAddresses []string
}

type certManager struct {
	ctx             context.Context
	kcpClient       client.Client
	skrClient       client.Client
	kyma            *v1alpha1.Kyma
	certificateName string
	secretName      string
}

func NewCertManager(kcpClient, skrClient client.Client, kyma *v1alpha1.Kyma) (*certManager, error) {
	if kcpClient == nil || skrClient == nil || kyma == nil {
		return nil, fmt.Errorf("coulc not create CertManager, clients or Kyma must not be empty")
	}
	return &certManager{
		ctx:             nil,
		kcpClient:       kcpClient,
		skrClient:       skrClient,
		kyma:            kyma,
		certificateName: fmt.Sprintf("%s-watcher-certificate", kyma.Name),
		secretName:      fmt.Sprintf("%s-watcher-certificate", kyma.Name),
	}, nil
}

// TODO: Check if it exists
func (c *certManager) Create() error {
	// Check if Certificate exists
	exists, err := c.exists()
	if exists {
		// TODO: check if cert is ready
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

func (c *certManager) exists() (bool, error) {
	cert := v1.Certificate{}
	err := c.kcpClient.Get(c.ctx, types.NamespacedName{
		Namespace: c.kyma.Namespace,
		Name:      c.certificateName,
	}, &cert, nil)
	if k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (c *certManager) createCertificate(
	ctx context.Context, certNamespace string,
	subjectAltName *SubjectAltName,
) error {
	//What happens on renewal? How should we deal with it
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
			// TODO: Maybe add subject (v1.X509Subject)
			DNSNames:       subjectAltName.DNSNames,
			IPAddresses:    subjectAltName.IPAddresses,
			URIs:           subjectAltName.URIs,
			EmailAddresses: subjectAltName.EmailAddresses,
			SecretName:     c.secretName, // Name of the secret which stored certificate
			SecretTemplate: &v1.CertificateSecretTemplate{
				Labels: secretLabels,
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
				Encoding:       privateKeyEncoding,
				Algorithm:      privateKeyAlgorithm,
			},
		},
	}

	return c.kcpClient.Create(ctx, &cert, nil)
}

func (c *certManager) getSubjectAltNames() (*SubjectAltName, error) {
	if domain, ok := c.kyma.Annotations[domainAnnotation]; ok {
		if domain == "" {
			return nil, fmt.Errorf("Domain-Annotation of KymaCR %s is empty", c.kyma.Name)
		}
		return &SubjectAltName{
			DNSNames: []string{domain},
		}, nil
	}
	return nil, fmt.Errorf("kymaCR %s does not contain annotation %s with SKR domain",
		c.kyma.Name, domainAnnotation)
}

// TODO double check, if we can use self-signed Issuer with `lifecycle-manager` label
func (c *certManager) getIssuer() (*v1.Issuer, error) {
	issuerList := &v1.IssuerList{}
	err := c.kcpClient.List(c.ctx, issuerList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{"app.kubernetes.io/name": "lifecycle-manager"}),
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

// TODO: Remove or move to secret watcher
func (c *certManager) syncSecret(ctx context.Context, certName, certNamespace string) error {

	interval := 1 * time.Second
	timeout := 5 * time.Second
	var certificate v1.Certificate

	conditionFunc := func(context.Context) (done bool, err error) {
		var cert v1.Certificate
		err = c.kcpClient.Get(ctx, types.NamespacedName{Name: certName, Namespace: certNamespace}, &cert)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// TODO: ausprobieren wie viel conditions es gibt und wie diese sich verhalten
		for _, condition := range cert.Status.Conditions {
			if condition.Type == v1.CertificateConditionReady {
				if condition.Status == (metav1.ConditionTrue) {
					certificate = cert
					return true, nil
				}
			}
		}
		return false, nil
	}
	// Wait until Certificate is ready
	err := apimachinerywait.PollImmediateWithContext(ctx, interval, timeout, conditionFunc)
	if err != nil {
		return err
	}

	// Get Secret containing PrivateKey and Certificate
	var certSecret corev1.Secret
	err = c.kcpClient.Get(ctx, types.NamespacedName{Name: certificate.Spec.SecretName, Namespace: certNamespace}, &certSecret)
	if err != nil {
		return err
	}
	// Copy secret to SKR cluster
	err = c.skrClient.Create(ctx, &certSecret, nil)
	if err != nil {
		return fmt.Errorf("could not create certificate secret on remote cluster: %w", err)
	}
	// Delete secret from KCP cluster
	err = c.kcpClient.Delete(ctx, &certSecret)
	if err != nil {
		return fmt.Errorf("could not delete certificate secret on local cluster: %w", err)
	}
	return nil
}
