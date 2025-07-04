package keychainprovider

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
)

type DefaultKeychainProvider struct{}

func NewDefaultKeyChainProvider() *DefaultKeychainProvider {
	return &DefaultKeychainProvider{}
}

func (a *DefaultKeychainProvider) Get(_ context.Context) (authn.Keychain, error) {
	return authn.NewMultiKeychain(google.Keychain, authn.DefaultKeychain), nil
}
