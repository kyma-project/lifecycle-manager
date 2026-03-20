package setup_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"

	"github.com/kyma-project/lifecycle-manager/internal/setup"
)

type mockSecretGetter struct {
	mock.Mock
}

func (m *mockSecretGetter) Get(ctx context.Context, name string,
) (*apicorev1.Secret, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(*apicorev1.Secret), args.Error(1)
}

func TestNewOCIRegistry_ReturnsError_WhenSecretRepoNil(t *testing.T) {
	_, err := setup.NewOCIRegistry(t.Context(), nil, "host", "", "")
	require.ErrorIs(t, err, setup.ErrSecretRepoNil)
}

func TestNewOCIRegistry_Success(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "")
	require.NoError(t, err)
	require.NotNil(t, ociRegistry)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenRegistryAndCredSecretEmpty(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "", "")
	require.ErrorIs(t, err, setup.ErrRegistryAndCredSecretEmpty)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenBothRegistryAndCredSecret(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "host", "secret", "")
	require.ErrorIs(t, err, setup.ErrBothRegistryAndCredSecret)
}

func TestOCIRegistry_Resolve_WithRegistry(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "")
	require.NoError(t, err)
	require.Equal(t, "myhost", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "https://myhost", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(),
		mockSecretGettr,
		"https://myhost/existing-path",
		"",
		"kyma-modules",
	)
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path/", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path", "", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithRegistry_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path/", "", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_Success(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("https://myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("http://myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestOCIRegistry_Resolve_WithCredSecret_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getValidDockerConfigJson("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
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

	unexpectedError := errors.New("unexpected error")
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return((*apicorev1.Secret)(nil),
		unexpectedError).Once()
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, unexpectedError)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenSecretIsNotDockerConfig(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	secret := &apicorev1.Secret{Data: map[string][]byte{"other": {}}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrSecretMissingDockerConfig)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenCredSecretMalformed(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": []byte("notjson")}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal .dockerconfigjson")
}

func TestOCIRegistry_Resolve_ReturnsError_WhenCredSecretHasNoURLs(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	jsonDockerConfig, err := getDockerConfigJsonWithNoEntries()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	_, err = setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrExactlyOneRegistryExpected)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenCredSecretHasMultipleEntries(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	jsonDockerConfig, err := getDockerConfigJsonWithMultipleEntries()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err = setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrExactlyOneRegistryExpected)
}

func TestOCIRegistry_Resolve_ReturnsError_WhenCredSecretHasNoEntries(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	jsonDockerConfig, err := getDockerConfigJsonWithEmptyStringEntry()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err = setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrRegistryMustNotBeEmpty)
}

func TestOCIRegistry_IsInsecure_WithHttpScheme(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "http://myhost", "", "")
	require.NoError(t, err)
	require.True(t, ociRegistry.IsInsecure())
}

func TestOCIRegistry_IsInsecure_WithHttpsScheme(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "https://myhost", "", "")
	require.NoError(t, err)
	require.False(t, ociRegistry.IsInsecure())
}

func TestOCIRegistry_IsInsecure_WithoutScheme(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "")
	require.NoError(t, err)
	require.False(t, ociRegistry.IsInsecure())
}

func getDockerConfigJsonWithNoEntries() ([]byte, error) {
	dockerConfig := map[string]any{"auths": map[string]any{}}
	return json.Marshal(dockerConfig)
}

func getDockerConfigJsonWithMultipleEntries() ([]byte, error) {
	dockerConfig := map[string]any{
		"auths": map[string]any{
			"registry1.io": map[string]string{"auth": "dXNlcjpwYXNz"},
			"registry2.io": map[string]string{"auth": "dXNlcjpwYXNz"},
		},
	}
	return json.Marshal(dockerConfig)
}

func getDockerConfigJsonWithEmptyStringEntry() ([]byte, error) {
	dockerConfig := map[string]any{
		"auths": map[string]any{
			"": map[string]string{},
		},
	}
	return json.Marshal(dockerConfig)
}
