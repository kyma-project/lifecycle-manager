package keychainprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/v1/google"
	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNoAuthSecretFound = errors.New("no private oci registry auth secret found")

type FromSecretKeychainProvider struct {
	kcpClient  client.Reader
	secretName types.NamespacedName
}

func NewFromSecretKeyChainProvider(
	kcpClient client.Reader,
	secretName types.NamespacedName,
) *FromSecretKeychainProvider {
	return &FromSecretKeychainProvider{kcpClient: kcpClient, secretName: secretName}
}

func (a *FromSecretKeychainProvider) Get(ctx context.Context) (authn.Keychain, error) {
	credSecret := apicorev1.Secret{}
	err := a.kcpClient.Get(ctx, a.secretName, &credSecret)
	if util.IsNotFound(err) {
		return nil, fmt.Errorf("%w: %s", ErrNoAuthSecretFound, a.secretName.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get oci cred secret: %w", err)
	}

	keychain, err := kubernetes.NewFromPullSecrets(ctx, []apicorev1.Secret{credSecret})
	if err != nil {
		return nil, fmt.Errorf("failed to create auth keychain: %w", err)
	}

	return authn.NewMultiKeychain(google.Keychain, keychain), nil
}
