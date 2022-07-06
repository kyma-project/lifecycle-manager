package signature

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/signatures"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrPublicKeyWrongType = errors.New("parsed public key is not correct type")

type MultiVerifier struct {
	verifiers map[string]signatures.Verifier
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
	ctx context.Context, c client.Client, validSignatureNames []string, namespace string,
) (*MultiVerifier, error) {
	secretList := &v1.SecretList{}

	selector, err := k8slabels.Parse(fmt.Sprintf("%s in (%s)", labels.Signature, strings.Join(validSignatureNames, ",")))
	if err != nil {
		return nil, err
	}

	if err := c.List(ctx, secretList, &client.ListOptions{
		LabelSelector: selector, Namespace: namespace,
	}); err != nil {
		return nil, err
	} else if len(secretList.Items) < 1 {
		gr := v1.SchemeGroupVersion.WithResource(fmt.Sprintf("secrets with label %s", labels.KymaName)).GroupResource()
		return nil, k8serrors.NewNotFound(gr, selector.String())
	}

	publicKeys := make(map[string]*rsa.PublicKey)
	for _, item := range secretList.Items {
		publicKey := item.Data["key"]
		block, _ := pem.Decode(publicKey)
		if block == nil {
			return nil, errors.New("unable to decode pem formatted block in key from secret")
		}
		untypedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse public key: %w", err)
		}
		switch key := untypedKey.(type) {
		case *rsa.PublicKey:
			publicKeys[item.Labels[labels.Signature]] = key
		default:
			return nil, fmt.Errorf("public key error: %w - type is %T", ErrPublicKeyWrongType, key)
		}
	}
	return CreateMultiRSAVerifier(publicKeys)
}
