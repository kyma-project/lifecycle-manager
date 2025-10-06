package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretGetter interface {
	Get(ctx context.Context, name string, opts apimetav1.GetOptions) (*apicorev1.Secret, error)
}

type OCIRegistryHostProvider struct {
	secretGetter   SecretGetter
	host           string
	credSecretName string
}

var (
	ErrSecretGetterNil           = errors.New("secretGetter cannot be nil")
	ErrHostAndCredSecretEmpty    = errors.New("host and credSecretName cannot be empty")
	ErrBothHostAndCredSecret     = errors.New("either host or credSecretName should be provided, not both")
	ErrSecretMissingDockerConfig = errors.New("secret missing .dockerconfigjson field")
	ErrNoRegistryHostFound       = errors.New("no registry host found in the credential secret")
)

func NewOCIRegistryHostProvider(
	secretGetter SecretGetter,
	host string,
	credSecretName string,
) (*OCIRegistryHostProvider, error) {
	if secretGetter == nil {
		return nil, ErrSecretGetterNil
	}
	if host == "" && credSecretName == "" {
		return nil, ErrHostAndCredSecretEmpty
	}
	if host != "" && credSecretName != "" {
		return nil, ErrBothHostAndCredSecret
	}

	return &OCIRegistryHostProvider{
		secretGetter:   secretGetter,
		host:           host,
		credSecretName: credSecretName,
	}, nil
}

func (oci *OCIRegistryHostProvider) ResolveHost(ctx context.Context) (string, error) {
	if oci.host != "" {
		return oci.host, nil
	}

	return oci.getHostFromCredSecret(ctx)
}

func (oci *OCIRegistryHostProvider) getHostFromCredSecret(ctx context.Context) (string, error) {
	secret, err := oci.secretGetter.Get(ctx, oci.credSecretName, apimetav1.GetOptions{})
	if err != nil {
		return "", err
	}
	data, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return "", ErrSecretMissingDockerConfig
	}

	var dockerConfig struct {
		Auths map[string]interface{} `json:"auths"`
	}
	if err := json.Unmarshal(data, &dockerConfig); err != nil {
		return "", fmt.Errorf("failed to unmarshal docker config json: %w", err)
	}

	for host := range dockerConfig.Auths {
		if host != "" {
			return host, nil
		}
	}
	return "", ErrNoRegistryHostFound
}
