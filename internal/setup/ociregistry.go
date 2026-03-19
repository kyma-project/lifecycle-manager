package setup

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	httpsSchemePrefix = "https://"
	httpSchemePrefix  = "http://"
)

var schemesToTrim = []string{
	httpsSchemePrefix,
	httpSchemePrefix,
}

type SecretRepository interface {
	Get(ctx context.Context, name string, opts apimetav1.GetOptions) (*apicorev1.Secret, error)
}

// OCIRegistry is a setup helper that resolves the OCI registry based on the provided registry configuration.
type OCIRegistry struct {
	secretRepo SecretRepository
}

var (
	ErrSecretRepoNil                 = errors.New("secretRepo cannot be nil")
	ErrRegistryAndCredSecretEmpty    = errors.New("registry and credSecretName cannot both be empty")
	ErrBothRegistryAndCredSecret     = errors.New("either registry or credSecretName should be provided, not both")
	ErrFailedToResolveHostFromSecret = errors.New("failed to resolve registry from credential secret")
	ErrFailedToGetRegistrySecret     = errors.New("failed to get registry credential secret")
	ErrSecretMissingDockerConfig     = errors.New("secret missing .dockerconfigjson field")
	ErrFailedToUnmarshalDockerConfig = errors.New("failed to unmarshal .dockerconfigjson")
	ErrMoreThanOneRegistryFound      = errors.New("more than one registry found in the credential secret")
	ErrNoRegistryFound               = errors.New("no registry found in .dockerconfigjson")
)

func NewOCIRegistry(secretRepo SecretRepository) (*OCIRegistry, error) {
	if secretRepo == nil {
		return nil, ErrSecretRepoNil
	}

	return &OCIRegistry{
		secretRepo: secretRepo,
	}, nil
}

// Resolve resolves the OCI registry reference based on the provided parameters.
// Only one of registry or registryCredSecretName must be provided. If both are provided, an error is returned.
// If registryCredSecretName is provided, the registry is extracted from the specified Kubernetes secret.
// The subPath is appended to the registry reference if it is not empty.
// It returns the resolved registry reference (host + optional path) without scheme.
func (oci *OCIRegistry) Resolve(ctx context.Context,
	registry string,
	registryCredSecretName string,
	subPath string,
) (string, error) {
	registry, err := oci.getRegistry(ctx, registry, registryCredSecretName)
	if err != nil {
		return "", err
	}

	registry = trimScheme(registry)

	// String concatenation is used explicitly
	// url.JoinPath may introduce problems due to unwanted URL encoding
	// path.Join may introduce problems if the registry reference contains a port
	if subPath != "" {
		registry = strings.TrimRight(registry, "/") + "/" + strings.TrimLeft(subPath, "/")
	}

	return registry, nil
}

// IsInsecure checks if the registry is insecure (i.e., uses http scheme) based on the provided parameters.
// Only one of registry or registryCredSecretName must be provided. If both are provided, an error is returned.
// If registryCredSecretName is provided, the registry is extracted from the specified Kubernetes secret.
func (oci *OCIRegistry) IsInsecure(ctx context.Context, registry string, registryCredSecretName string) (bool, error) {
	registry, err := oci.getRegistry(ctx, registry, registryCredSecretName)
	if err != nil {
		return false, err
	}

	return strings.HasPrefix(registry, httpSchemePrefix), nil
}

func (oci *OCIRegistry) getRegistry(ctx context.Context, registry string, registryCredSecretName string) (string, error) {
	if registry == "" && registryCredSecretName == "" {
		return "", ErrRegistryAndCredSecretEmpty
	}
	if registry != "" && registryCredSecretName != "" {
		return "false", ErrBothRegistryAndCredSecret
	}

	if registry != "" {
		return registry, nil
	}

	registry, err := oci.getRegistryFromCredSecret(ctx, registryCredSecretName)
	if err != nil {
		return "", errors.Join(ErrFailedToResolveHostFromSecret, err)
	}

	return registry, nil
}

func (oci *OCIRegistry) getRegistryFromCredSecret(ctx context.Context, credSecretName string) (string, error) {
	secret, err := oci.secretRepo.Get(ctx, credSecretName, apimetav1.GetOptions{})
	if err != nil {
		return "", errors.Join(ErrFailedToGetRegistrySecret, err)
	}

	data, ok := secret.Data[apicorev1.DockerConfigJsonKey]
	if !ok {
		return "", ErrSecretMissingDockerConfig
	}

	var dockerConfig struct {
		Auths map[string]any `json:"auths"`
	}
	if err := json.Unmarshal(data, &dockerConfig); err != nil {
		return "", errors.Join(ErrFailedToUnmarshalDockerConfig, err)
	}

	if len(dockerConfig.Auths) > 1 {
		return "", ErrMoreThanOneRegistryFound
	}

	if len(dockerConfig.Auths) == 0 {
		return "", ErrNoRegistryFound
	}

	for registry := range dockerConfig.Auths {
		if registry != "" {
			return registry, nil
		}
	}
	return "", ErrNoRegistryFound
}

func trimScheme(s string) string {
	for _, scheme := range schemesToTrim {
		if r, found := strings.CutPrefix(s, scheme); found {
			return r
		}
	}
	return s
}
