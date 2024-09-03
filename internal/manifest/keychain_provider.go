package manifest

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type KeyChainProvider struct {
	kcpClient client.Client
}

func NewKeyChainProvider(kcpClient client.Client) *KeyChainProvider {
	return &KeyChainProvider{kcpClient: kcpClient}
}

func (a *KeyChainProvider) Get(ctx context.Context, imageSpec v1beta2.ImageSpec) (authn.Keychain, error) {
	var keyChain authn.Keychain
	var err error
	if imageSpec.CredSecretSelector == nil {
		keyChain = authn.DefaultKeychain
	} else if keyChain, err = ocmextensions.GetAuthnKeychain(ctx, imageSpec.CredSecretSelector, a.kcpClient); err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(google.Keychain, keyChain), nil
}
