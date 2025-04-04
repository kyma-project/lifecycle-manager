package certmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher/certificate"
)

//nolint:gochecknoglobals // this is const config
var certificateLabels = k8slabels.Set{
	shared.PurposeLabel: shared.CertManager,
	shared.ManagedBy:    shared.OperatorName,
}

var ErrNoRenewalTime = errors.New("no renewal time set for certificate")

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
			Duration:    &apimetav1.Duration{Duration: c.config.Duration},
			RenewBefore: &apimetav1.Duration{Duration: c.config.RenewBefore},
			DNSNames:    dnsNames,
			SecretName:  name,
			SecretTemplate: &certmanagerv1.CertificateSecretTemplate{
				Labels: certificateLabels,
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

	if err := c.kcpClient.Delete(ctx, cert); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete certificate %s-%s: %w", name, namespace, err)
	}

	return nil
}

func (c *CertificateClient) GetRenewalTime(ctx context.Context,
	name string,
	namespace string,
) (time.Time, error) {
	cert := &certmanagerv1.Certificate{}
	cert.SetName(name)
	cert.SetNamespace(namespace)

	if err := c.kcpClient.Get(ctx, client.ObjectKeyFromObject(cert), cert); err != nil {
		return time.Time{}, fmt.Errorf("failed to get certificate %s-%s: %w", name, namespace, err)
	}

	if cert.Status.RenewalTime == nil || cert.Status.RenewalTime.Time.IsZero() {
		return time.Time{}, ErrNoRenewalTime
	}

	return cert.Status.RenewalTime.Time, nil
}
