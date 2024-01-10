package signature

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	ocmmetav1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	"github.com/open-component-model/ocm/pkg/signing"
	"github.com/open-component-model/ocm/pkg/signing/handlers/rsa"
	"github.com/open-component-model/ocm/pkg/signing/signutils"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

var ErrNoSignatureFound = errors.New("no signature was found")

const ValidSignatureName = "kyma-module-signature"

type Verifier interface {
	Verify(componentDescriptor *compdesc.ComponentDescriptor, signature ocmmetav1.Signature) error
}

type MultiVerifier struct {
	registry signing.Registry
}

type VerificationSettings struct {
	client.Client
	PublicKeyFilePath  string
	EnableVerification bool
}

type Verification func(descriptor *compdesc.ComponentDescriptor) error

func Verify(
	descriptor *compdesc.ComponentDescriptor, signatureVerification Verification,
) error {
	if err := signatureVerification(descriptor); err != nil {
		return fmt.Errorf("signature verification error, untrusted: %w", err)
	}
	return nil
}

func NewNoSignatureVerification() func(descriptor *compdesc.ComponentDescriptor) error {
	var NoSignatureVerification Verification = func(descriptor *compdesc.ComponentDescriptor) error { return nil }
	return NoSignatureVerification
}

func NewVerification(
	ctx context.Context,
	clnt client.Client,
	enableVerification bool,
	publicKeyFilePath,
	moduleName string,
) (Verification, error) {
	if !enableVerification {
		return NewNoSignatureVerification(), nil
	}

	var verifier Verifier
	var err error
	if publicKeyFilePath == "" {
		verifier, err = CreateRSAVerifierFromSecrets(ctx, clnt, moduleName)
	} else {
		verifier, err = CreateRSAVerifierFromPublicKeyFile(publicKeyFilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("error occurred while initializing Signature Verifier: %w", err)
	}

	return func(descriptor *compdesc.ComponentDescriptor) error {
		for _, sig := range descriptor.Signatures {
			if sig.Name == ValidSignatureName {
				if err := verifier.Verify(descriptor, sig); err != nil {
					return fmt.Errorf("error occurred during signature verification: %w", err)
				}
				return nil
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

func (v MultiVerifier) Verify(descriptor *compdesc.ComponentDescriptor, signature ocmmetav1.Signature) error {
	err := compdesc.Verify(descriptor, v.registry, signature.Name)
	if err != nil {
		return fmt.Errorf("failed to verify descriptor signature: %w", err)
	}
	return nil
}

// CreateRSAVerifierFromSecrets creates an instance of RsaVerifier from a rsa public key file located as secret
// in kubernetes. The key has to be in the PKIX, ASN.1 DER form, see x509.ParsePKIXPublicKey.
func CreateRSAVerifierFromSecrets(
	ctx context.Context,
	k8sClient client.Client,
	moduleName string,
) (*MultiVerifier, error) {
	secretList := &apicorev1.SecretList{}

	secretSelector := &apimetav1.LabelSelector{
		MatchLabels: k8slabels.Set{shared.Signature: ValidSignatureName, shared.ModuleName: moduleName},
	}
	selector, err := apimetav1.LabelSelectorAsSelector(secretSelector)
	if err != nil {
		return nil, fmt.Errorf("error converting signature labelSelector: %w", err)
	}
	if err = k8sClient.List(ctx, secretList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	} else if len(secretList.Items) < 1 {
		gr := apicorev1.SchemeGroupVersion.WithResource(fmt.Sprintf("secrets with label %s",
			shared.KymaName)).GroupResource()
		return nil, apierrors.NewNotFound(gr, selector.String())
	}
	registry := signing.NewKeyRegistry()
	for _, item := range secretList.Items {
		publicKey := item.Data["key"]
		key, err := signutils.ParsePublicKey(publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key: %w", err)
		}
		registry.RegisterPublicKey(ValidSignatureName, key)
		registry.RegisterPublicKey(item.Labels[shared.Signature], key)
	}
	return CreateMultiRSAVerifier(registry)
}

func CreateRSAVerifierFromPublicKeyFile(file string) (*MultiVerifier, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}
	registry := signing.NewKeyRegistry()
	key, err := signutils.ParsePublicKey(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	registry.RegisterPublicKey(ValidSignatureName, key)
	return CreateMultiRSAVerifier(registry)
}
