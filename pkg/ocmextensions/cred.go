package ocmextensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/go-containerregistry/pkg/authn"
	authnK8s "github.com/google/go-containerregistry/pkg/authn/kubernetes"
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

func NoSchemeURL(url string) string {
	regex := regexp.MustCompile(`^https?://`)
	return regex.ReplaceAllString(url, "")
}
