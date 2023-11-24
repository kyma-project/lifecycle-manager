package ocmextensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/kubernetes"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNoAuthSecretFound = errors.New("no auth secret found")

//nolint:ireturn //external dependency used here already returns an interface
func GetAuthnKeychain(ctx context.Context,
	credSecretSelector *apimetav1.LabelSelector,
	clnt client.Client,
) (authn.Keychain, error) {
	secretList, err := getCredSecrets(ctx, credSecretSelector, clnt)
	if err != nil {
		return nil, err
	}
	keychain, err := kubernetes.NewFromPullSecrets(ctx, secretList.Items)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth keychain: %w", err)
	}
	return keychain, nil
}

func getCredSecrets(
	ctx context.Context,
	credSecretSelector *apimetav1.LabelSelector,
	clusterClient client.Client,
) (apicorev1.SecretList, error) {
	secretList := apicorev1.SecretList{}
	selector, err := apimetav1.LabelSelectorAsSelector(credSecretSelector)
	if err != nil {
		return secretList, fmt.Errorf("error converting labelSelector: %w", err)
	}
	err = clusterClient.List(
		ctx, &secretList, &client.ListOptions{
			LabelSelector: selector,
		},
	)
	if err != nil {
		return secretList, fmt.Errorf("failed to list cred secrets: %w", err)
	}
	if len(secretList.Items) == 0 {
		return secretList, ErrNoAuthSecretFound
	}
	return secretList, nil
}

func GenerateLabelSelector(registryCredValue []byte) (*apimetav1.LabelSelector, error) {
	credSecretLabel := make(map[string]string)
	if err := json.Unmarshal(registryCredValue, &credSecretLabel); err != nil {
		return nil, fmt.Errorf("failed to cred secret labels: %w", err)
	}
	return &apimetav1.LabelSelector{
		MatchLabels: credSecretLabel,
	}, nil
}

func NoSchemeURL(url string) string {
	regex := regexp.MustCompile(`^https?://`)
	return regex.ReplaceAllString(url, "")
}
