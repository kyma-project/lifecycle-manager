package ocmextensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	authnK8s "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/open-component-model/ocm/pkg/contexts/credentials"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoAuthSecretFound = errors.New("no auth secret found")

func GetAuthnKeychain(ctx context.Context,
	credSecretSelector *metav1.LabelSelector,
	clnt client.Client,
) (authn.Keychain, error) {
	secretList, err := getCredSecrets(ctx, credSecretSelector, clnt)
	if err != nil {
		return nil, err
	}
	return authnK8s.NewFromPullSecrets(ctx, secretList.Items)
}

func GetCredentials(ctx context.Context,
	registryCredValue []byte,
	ociRegistry *OCIRegistry,
	clnt client.Client,
) (credentials.Credentials, error) {
	credSecretSelector, err := GenerateLabelSelector(registryCredValue)
	if err != nil {
		return nil, err
	}
	secretList, err := getCredSecrets(ctx, credSecretSelector, clnt)
	if err != nil {
		return nil, err
	}
	keychain, err := authnK8s.NewFromPullSecrets(ctx, secretList.Items)
	if err != nil {
		return nil, err
	}
	authenticator, err := keychain.Resolve(ociRegistry)
	if err != nil {
		return nil, err
	}
	authConfig, err := authenticator.Authorization()
	if err != nil {
		return nil, err
	}
	return credentials.DirectCredentials{
		"username": authConfig.Username,
		"password": authConfig.Password,
	}, nil
}

func getCredSecrets(
	ctx context.Context,
	credSecretSelector *metav1.LabelSelector,
	clusterClient client.Client,
) (corev1.SecretList, error) {
	secretList := corev1.SecretList{}
	selector, err := metav1.LabelSelectorAsSelector(credSecretSelector)
	if err != nil {
		return secretList, fmt.Errorf("error converting labelSelector: %w", err)
	}
	err = clusterClient.List(
		ctx, &secretList, &client.ListOptions{
			LabelSelector: selector,
		},
	)
	if err != nil {
		return secretList, err
	}
	if len(secretList.Items) == 0 {
		return secretList, ErrNoAuthSecretFound
	}
	return secretList, nil
}

func GenerateLabelSelector(registryCredValue []byte) (*metav1.LabelSelector, error) {
	credSecretLabel := make(map[string]string)
	if err := json.Unmarshal(registryCredValue, &credSecretLabel); err != nil {
		return nil, err
	}
	return &metav1.LabelSelector{
		MatchLabels: credSecretLabel,
	}, nil
}
