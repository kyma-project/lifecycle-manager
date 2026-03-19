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

	_, err := setup.NewOCIRegistry(nil)
	require.ErrorIs(t, err, setup.ErrSecretRepoNil)

	ociRegistry, err := setup.NewOCIRegistry(mockSecretGettr)
	require.NoError(t, err)
	require.NotNil(t, ociRegistry)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenRegistryAndCredSecretEmpty(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	_, err := ociRegistry.Resolve(t.Context(), "", "", "")
	require.ErrorIs(t, err, setup.ErrRegistryAndCredSecretEmpty)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenBothRegistryAndCredSecret(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	_, err := ociRegistry.Resolve(t.Context(), "host", "secret", "")
	require.ErrorIs(t, err, setup.ErrBothRegistryAndCredSecret)
}

func TestOCIRegistry_Resolve_WithRegistry(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "myhost", "", "")
	require.NoError(t, err)
	require.Equal(t, "myhost", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "myhost", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "myhost/existing-path", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "https://myhost", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "https://myhost/existing-path", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "myhost/existing-path/", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "myhost/existing-path", "", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithRegistry_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)
	registryRef, err := ociRegistry.Resolve(t.Context(), "myhost/existing-path/", "", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_Success(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("https://myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("http://myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", registryRef)
}

func TestOCIRegistry_Resolve_WithCredSecret_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", registryRef)
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

func TestOCIRegistry_Resolve_ReturnsError_WhenSecretGetterReturnsError(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	unexpectedError := errors.New("unexpected error")
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return((*apicorev1.Secret)(nil),
		unexpectedError).Once()
	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "")
	require.ErrorIs(t, err, unexpectedError)
	require.Empty(t, registryRef)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenSecretIsNotDockerConfig(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	secret := &apicorev1.Secret{Data: map[string][]byte{"other": {}}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrSecretMissingDockerConfig)
	require.Empty(t, registryRef)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenCredSecretMalformed(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": []byte("notjson")}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal docker config json")
	require.Empty(t, registryRef)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenCredSecretHasNoURLs(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretGettr)

	jsonDockerConfig, err := getEmptyDockerConfigJson()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	registryRef, err := ociRegistry.Resolve(t.Context(), "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrNoRegistryFound)
	require.Empty(t, registryRef)
}

func getEmptyDockerConfigJson() ([]byte, error) {
	dockerConfig := map[string]any{"auths": map[string]any{}}
	return json.Marshal(dockerConfig)
}
