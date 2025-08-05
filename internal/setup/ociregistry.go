package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretInterface interface {
	Get(ctx context.Context, name string, opts apimetav1.GetOptions) (*apicorev1.Secret, error)
}

type OCIRegistry struct {
	secretInterface SecretInterface
	host            string
	credSecretName  string
}

var (
	ErrSecretInterfaceNil        = errors.New("secretInterface cannot be nil")
	ErrHostAndCredSecretEmpty    = errors.New("host and credSecretName cannot be empty")
	ErrBothHostAndCredSecret     = errors.New("either host or credSecretName should be provided, not both")
	ErrSecretMissingDockerConfig = errors.New("secret missing .dockerconfigjson field")
	ErrNoRegistryHostFound       = errors.New("no registry host found in the credential secret")
)

func NewOCIRegistry(secretInterface SecretInterface, host string, credSecretName string) (*OCIRegistry, error) {
	if secretInterface == nil {
		return nil, ErrSecretInterfaceNil
	}
	if host == "" && credSecretName == "" {
		return nil, ErrHostAndCredSecretEmpty
	}
	if host != "" && credSecretName != "" {
		return nil, ErrBothHostAndCredSecret
	}

	return &OCIRegistry{
		secretInterface: secretInterface,
		host:            host,
		credSecretName:  credSecretName,
	}, nil
}

func (oci *OCIRegistry) ResolveHost(ctx context.Context) (string, error) {
	if oci.host != "" {
		return oci.host, nil
	}

	return oci.getHostFromCredSecret(ctx)
}

func (oci *OCIRegistry) getHostFromCredSecret(ctx context.Context) (string, error) {
	secret, err := oci.secretInterface.Get(ctx, oci.credSecretName, apimetav1.GetOptions{})
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
