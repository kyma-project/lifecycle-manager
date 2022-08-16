package signature

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/signatures"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrDecodePEMInSecret  = errors.New("unable to decode pem formatted block in key from secret")
	ErrPublicKeyWrongType = errors.New("parsed public key is not correct type")
	ErrNoSignatureFound   = errors.New("no signature was found")
)

type MultiVerifier struct {
	verifiers map[string]signatures.Verifier
}

type VerificationSettings struct {
	client.Client
	PublicKeyFilePath   string
	ValidSignatureNames []string
	EnableVerification  bool
}

type Verification func(descriptor *v2.ComponentDescriptor) error

var NoSignatureVerification Verification = func(descriptor *v2.ComponentDescriptor) error { return nil } //nolint:lll,gochecknoglobals

func Verify(
	descriptor *v2.ComponentDescriptor, signatureVerification Verification,
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

	var verifier signatures.Verifier
	var err error
	if settings.PublicKeyFilePath == "" {
		verifier, err = CreateRSAVerifierFromSecrets(ctx, settings, settings.ValidSignatureNames, namespace)
	} else {
		verifier, err = signatures.CreateRSAVerifierFromKeyFile(settings.PublicKeyFilePath)
	}
	if err != nil {
		return nil, fmt.Errorf("error occurred while initializing Signature Verifier: %w", err)
	}

	return func(descriptor *v2.ComponentDescriptor) error {
		for _, sig := range descriptor.Signatures {
			for _, validName := range settings.ValidSignatureNames {
				if sig.Name == validName {
					if err := verifier.Verify(*descriptor, sig); err != nil {
						return fmt.Errorf("error occurred during signature verification: %w", err)
					}
					return nil
				}
			}
		}
		return fmt.Errorf("descriptor contains invalid signature list: %w", ErrNoSignatureFound)
	}, nil
}

func CreateMultiRSAVerifier(publicKeys map[string]*rsa.PublicKey) (*MultiVerifier, error) {
	verifiers := make(map[string]signatures.Verifier)
	for signatureName, publicKey := range publicKeys {
		var err error
		verifiers[signatureName], err = signatures.CreateRSAVerifier(publicKey)
		if err != nil {
			return nil, err
		}
	}
	return &MultiVerifier{verifiers}, nil
}

func (v MultiVerifier) Verify(componentDescriptor v2.ComponentDescriptor, signature v2.Signature) error {
	return v.verifiers[signature.Name].Verify(componentDescriptor, signature) //nolint:wrapcheck
}

// CreateRSAVerifierFromSecrets creates an instance of RsaVerifier from a rsa public key file located as secret
// in kubernetes. The key has to be in the PKIX, ASN.1 DER form, see x509.ParsePKIXPublicKey.
func CreateRSAVerifierFromSecrets(
	ctx context.Context, k8sClient client.Client, validSignatureNames []string, namespace string,
) (*MultiVerifier, error) {
	secretList := &v1.SecretList{}

	selector, err := k8slabels.Parse(fmt.Sprintf("%s in (%s)", v1alpha1.Signature, strings.Join(validSignatureNames, ",")))
	if err != nil {
		return nil, err
	}

	if err := k8sClient.List(ctx, secretList, &client.ListOptions{
		LabelSelector: selector, Namespace: namespace,
	}); err != nil {
		return nil, err
	} else if len(secretList.Items) < 1 {
		gr := v1.SchemeGroupVersion.WithResource(fmt.Sprintf("secrets with label %s", v1alpha1.KymaName)).GroupResource()
		return nil, k8serrors.NewNotFound(gr, selector.String())
	}

	publicKeys := make(map[string]*rsa.PublicKey)
	for _, item := range secretList.Items {
		publicKey := item.Data["key"]
		block, _ := pem.Decode(publicKey)
		if block == nil {
			return nil, ErrDecodePEMInSecret
		}
		untypedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse public key: %w", err)
		}
		switch key := untypedKey.(type) {
		case *rsa.PublicKey:
			publicKeys[item.Labels[v1alpha1.Signature]] = key
		default:
			return nil, fmt.Errorf("public key error: %w - type is %T", ErrPublicKeyWrongType, key)
		}
	}
	return CreateMultiRSAVerifier(publicKeys)
}
