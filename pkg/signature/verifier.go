package signature

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	metav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/signing"
	"github.com/open-component-model/ocm/pkg/signing/handlers/rsa"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoSignatureFound = errors.New("no signature was found")

type Verifier interface {
	Verify(componentDescriptor *compdesc.ComponentDescriptor, signature metav1.Signature) error
}

type MultiVerifier struct {
	registry signing.Registry
}

type VerificationSettings struct {
	client.Client
	PublicKeyFilePath   string
	ValidSignatureNames []string
	EnableVerification  bool
}

type Verification func(descriptor *compdesc.ComponentDescriptor) error

var NoSignatureVerification Verification = func(descriptor *compdesc.ComponentDescriptor) error { return nil } //nolint:lll,gochecknoglobals

func Verify(
	descriptor *compdesc.ComponentDescriptor, signatureVerification Verification,
) error {
	if err := signatureVerification(descriptor); err != nil {
		return fmt.Errorf("signature verification error, untrusted: %w", err)
	}
	return nil
}

func (settings *VerificationSettings) NewVerification(
	ctx context.Context, namespace string,
) (Verification, error) {
	if !settings.EnableVerification {
		return NoSignatureVerification, nil
	}

	var verifier Verifier
	var err error
	if settings.PublicKeyFilePath == "" {
		verifier, err = CreateRSAVerifierFromSecrets(ctx, settings.Client, settings.ValidSignatureNames, namespace)
	} else {
		verifier, err = CreateRSAVerifierFromPublicKeyFile(settings.PublicKeyFilePath, settings.ValidSignatureNames)
	}
	if err != nil {
		return nil, fmt.Errorf("error occurred while initializing Signature Verifier: %w", err)
	}

	return func(descriptor *compdesc.ComponentDescriptor) error {
		for _, sig := range descriptor.Signatures {
			for _, validName := range settings.ValidSignatureNames {
				if sig.Name == validName {
					if err := verifier.Verify(descriptor, sig); err != nil {
						return fmt.Errorf("error occurred during signature verification: %w", err)
					}
					return nil
				}
			}
		}
		return fmt.Errorf("descriptor contains invalid signature list: %w", ErrNoSignatureFound)
	}, nil
}

func CreateMultiRSAVerifier(keys signing.KeyRegistry) (*MultiVerifier, error) {
	handlers := signing.NewHandlerRegistry()
	handlers.RegisterVerifier(rsa.Algorithm, rsa.Handler{})
	for _, hasher := range signing.DefaultHandlerRegistry().HasherNames() {
		handlers.RegisterHasher(signing.DefaultHandlerRegistry().GetHasher(hasher))
	}
	return &MultiVerifier{registry: signing.NewRegistry(handlers, keys)}, nil
}

func (v MultiVerifier) Verify(descriptor *compdesc.ComponentDescriptor, signature metav1.Signature) error {
	return compdesc.Verify(descriptor, v.registry, signature.Name)
}

// CreateRSAVerifierFromSecrets creates an instance of RsaVerifier from a rsa public key file located as secret
// in kubernetes. The key has to be in the PKIX, ASN.1 DER form, see x509.ParsePKIXPublicKey.
func CreateRSAVerifierFromSecrets(
	ctx context.Context, k8sClient client.Client, validSignatureNames []string, namespace string,
) (*MultiVerifier, error) {
	secretList := &v1.SecretList{}

	selector, err := k8slabels.Parse(fmt.Sprintf("%s in (%s)", v1beta1.Signature, strings.Join(validSignatureNames, ",")))
	if err != nil {
		return nil, err
	}

	if err := k8sClient.List(ctx, secretList, &client.ListOptions{
		LabelSelector: selector, Namespace: namespace,
	}); err != nil {
		return nil, err
	} else if len(secretList.Items) < 1 {
		gr := v1.SchemeGroupVersion.WithResource(fmt.Sprintf("secrets with label %s", v1beta1.KymaName)).GroupResource()
		return nil, k8serrors.NewNotFound(gr, selector.String())
	}
	registry := signing.NewKeyRegistry()
	for _, item := range secretList.Items {
		publicKey := item.Data["key"]
		key, err := signing.ParsePublicKey(publicKey)
		if err != nil {
			return nil, err
		}
		registry.RegisterPublicKey(item.Labels[v1beta1.Signature], key)
	}
	return CreateMultiRSAVerifier(registry)
}

func CreateRSAVerifierFromPublicKeyFile(
	file string, validSignatureNames []string,
) (*MultiVerifier, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	registry := signing.NewKeyRegistry()
	key, err := signing.ParsePublicKey(data)
	if err != nil {
		return nil, err
	}
	for _, signatureName := range validSignatureNames {
		registry.RegisterPublicKey(signatureName, key)
	}
	return CreateMultiRSAVerifier(registry)
}
