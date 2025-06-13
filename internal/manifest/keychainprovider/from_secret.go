package keychainprovider

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn/kubernetes"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type FromSecretKeychainProvider struct {
	kcpClient  client.Client
	secretName types.NamespacedName
}

func NewFromSecretKeyChainProvider(kcpClient client.Client, secretName types.NamespacedName) *FromSecretKeychainProvider {
	return &FromSecretKeychainProvider{kcpClient: kcpClient, secretName: secretName}
}

func (a *FromSecretKeychainProvider) Get(ctx context.Context, imageSpec v1beta2.ImageSpec) (authn.Keychain, error) {
	credSecret := apicorev1.Secret{}
	err := a.kcpClient.Get(ctx, a.secretName, &credSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get oci cred secret: %w", err)
	}
	keychain, err := kubernetes.NewFromPullSecrets(ctx, []apicorev1.Secret{credSecret})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth keychain: %w", err)
	}

	return authn.NewMultiKeychain(google.Keychain, keychain), nil
}
