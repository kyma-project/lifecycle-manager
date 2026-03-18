package setup_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/internal/setup"
)

type mockSecretGetter struct {
	mock.Mock
}

func (m *mockSecretGetter) Get(ctx context.Context, name string, opts apimetav1.GetOptions,
) (*apicorev1.Secret, error) {
	args := m.Called(ctx, name, opts)
	return args.Get(0).(*apicorev1.Secret), args.Error(1)
}

func TestNewOCIRegistry(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	_, err := setup.NewOCIRegistryHostProvider(nil, "", "", "")
	require.ErrorIs(t, err, setup.ErrSecretGetterNil)

	_, err = setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "", "")
	require.ErrorIs(t, err, setup.ErrHostAndCredSecretEmpty)

	_, err = setup.NewOCIRegistryHostProvider(mockSecretGettr, "host", "secret", "")
	require.ErrorIs(t, err, setup.ErrBothHostAndCredSecret)

	ociRegistry, err := setup.NewOCIRegistryHostProvider(mockSecretGettr, "host", "", "")
	require.NoError(t, err)
	require.NotNil(t, ociRegistry)

	ociRegistry, err = setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "secret", "")
	require.NoError(t, err)
	require.NotNil(t, ociRegistry)
}

func TestOCIRegistry_ResolveHost_WithHost(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "myhost", "", "")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myhost", host)
}

func TestOCIRegistry_ResolveHost_WithHost_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "myhost", "", "kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithHost_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "myhost/existing-path", "", "kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithHost_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "https://myhost", "", "kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "https://myhost/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithHost_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(
		mockSecretGettr,
		"https://myhost/existing-path",
		"",
		"kyma-modules",
	)
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "https://myhost/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithHost_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "myhost/existing-path/", "", "kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithHost_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "myhost/existing-path", "", "/kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithHost_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "myhost/existing-path/", "", "/kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_Success(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "")

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myregistry.io", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("https://myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "https://myregistry.io/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("https://myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "https://myregistry.io/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "/kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", host)
}

func TestOCIRegistry_ResolveHost_WithCredSecret_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "/kyma-modules")

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", host)
}

func getValidDockerConfigJson(hostName string) ([]byte, error) {
	dockerConfig := map[string]any{
		"auths": map[string]any{
			hostName: map[string]string{"auth": "dXNlcjpwYXNz"},
		},
	}
	jsonDockerConfig, err := json.Marshal(dockerConfig)
	return jsonDockerConfig, err
}

func TestOCIRegistry_ResolveHost_ReturnsError_WhenHostAndSubPathJoinFails(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "%invalid", "", "kyma-modules")
	host, err := ociRegistry.ResolveHost(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to join host with modules repository subpath")
	require.Empty(t, host)
}

func TestOCIRegistry_ResolveHost_ReturnsError_WhenSecretGetterReturnsError(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "")

	unexpectedError := errors.New("unexpected error")
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return((*apicorev1.Secret)(nil),
		unexpectedError).Once()
	host, err := ociRegistry.ResolveHost(t.Context())
	require.ErrorIs(t, err, unexpectedError)
	require.Empty(t, host)
}

func TestOCIRegistry_ResolveHost_ReturnsError_WhenSecretIsNotDockerConfig(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "")

	secret := &apicorev1.Secret{Data: map[string][]byte{"other": {}}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	host, err := ociRegistry.ResolveHost(t.Context())
	require.ErrorIs(t, err, setup.ErrSecretMissingDockerConfig)
	require.Empty(t, host)
}

func TestOCIRegistry_ResolveHost_ReturnsError_WhenCredSecretMalformed(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "")

	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": []byte("notjson")}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	host, err := ociRegistry.ResolveHost(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal docker config json")
	require.Empty(t, host)
}

func TestOCIRegistry_ResolveHost_ReturnsError_WhenCredSecretHasNotHosts(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistryHostProvider(mockSecretGettr, "", "mysecret", "")

	jsonDockerConfig, err := getEmptyDockerConfigJson()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	host, err := ociRegistry.ResolveHost(t.Context())
	require.ErrorIs(t, err, setup.ErrNoRegistryHostFound)
	require.Empty(t, host)
}

func getEmptyDockerConfigJson() ([]byte, error) {
	dockerConfig := map[string]any{"auths": map[string]any{}}
	return json.Marshal(dockerConfig)
}
