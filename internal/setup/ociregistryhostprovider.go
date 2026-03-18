package setup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretGetter interface {
	Get(ctx context.Context, name string, opts apimetav1.GetOptions) (*apicorev1.Secret, error)
}

type OCIRegistryHostProvider struct {
	secretGetter             SecretGetter
	host                     string
	credSecretName           string
	modulesRepositorySubPath string
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
	modulesRepositorySubPath string,
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
		secretGetter:             secretGetter,
		host:                     host,
		credSecretName:           credSecretName,
		modulesRepositorySubPath: modulesRepositorySubPath,
	}, nil
}

func (oci *OCIRegistryHostProvider) ResolveHost(ctx context.Context) (string, error) {
	var host string

	if oci.host != "" {
		host = oci.host
	} else {
		var err error
		host, err = oci.getHostFromCredSecret(ctx)
		if err != nil {
			return "", err
		}
	}

	// String concatenation is used explicitly
	// url.JoinPath may introduce problems due to unwanted URL encoding
	// path.Join may introduce problems if the host contains scheme or port
	if oci.modulesRepositorySubPath != "" {
		host = strings.TrimRight(host, "/") + "/" + strings.TrimLeft(oci.modulesRepositorySubPath, "/")
	}

	return host, nil
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
		Auths map[string]any `json:"auths"`
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
