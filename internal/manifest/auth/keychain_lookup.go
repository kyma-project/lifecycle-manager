package auth

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
)

func LookupKeyChain(ctx context.Context, imageSpec v1beta2.ImageSpec,
	targetClient client.Client,
) (authn.Keychain, error) {
	var keyChain authn.Keychain
	var err error
	if imageSpec.CredSecretSelector == nil {
		keyChain = authn.DefaultKeychain
	} else if keyChain, err = ocmextensions.GetAuthnKeychain(ctx, imageSpec.CredSecretSelector,
		targetClient); err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(google.Keychain, keyChain), nil
}
