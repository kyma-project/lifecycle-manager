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
	var secret *apicorev1.Secret
	if arg := args.Get(0); arg != nil {
		secret = arg.(*apicorev1.Secret)
	}
	return secret, args.Error(1)
}

func TestNewOCIRegistry_ReturnsError_WhenSecretRepoNil(t *testing.T) {
	_, err := setup.NewOCIRegistry(t.Context(), nil, "host", "", "")
	require.ErrorIs(t, err, setup.ErrSecretRepoNil)
}

func TestNewOCIRegistry_ReturnsError_WhenRegistryAndCredSecretEmpty(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "", "")
	require.ErrorIs(t, err, setup.ErrRegistryAndCredSecretEmpty)
}

func TestNewOCIRegistry_ReturnsError_WhenBothRegistryAndCredSecretProvided(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "host", "secret", "")
	require.ErrorIs(t, err, setup.ErrBothRegistryAndCredSecret)
}

func TestNewOCIRegistry_ReturnsError_WhenSecretGetterReturnsError(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	unexpectedError := errors.New("unexpected error")
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return((*apicorev1.Secret)(nil),
		unexpectedError).Once()
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, unexpectedError)
}

func TestNewOCIRegistry_ReturnsError_WhenSecretMissingDockerConfigKey(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	secret := &apicorev1.Secret{Data: map[string][]byte{"other": {}}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrSecretMissingDockerConfig)
}

func TestNewOCIRegistry_ReturnsError_WhenCredSecretMalformed(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": []byte("notjson")}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal .dockerconfigjson")
}

func TestNewOCIRegistry_ReturnsError_WhenCredSecretAuthsEmpty(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	jsonDockerConfig, err := getDockerConfigJsonWithNoEntries()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err = setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrExactlyOneRegistryExpected)
}

func TestNewOCIRegistry_ReturnsError_WhenCredSecretHasMultipleEntries(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	jsonDockerConfig, err := getDockerConfigJsonWithMultipleEntries()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err = setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrExactlyOneRegistryExpected)
}

func TestNewOCIRegistry_ReturnsError_WhenCredSecretHasEmptyRegistryName(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	jsonDockerConfig, err := getDockerConfigJsonWithEmptyStringEntry()
	require.NoError(t, err)
	secret := &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil).Once()
	_, err = setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.ErrorIs(t, err, setup.ErrRegistryMustNotBeEmpty)
}

func TestNewOCIRegistry_Success(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "")
	require.NoError(t, err)
	require.NotNil(t, ociRegistry)
}

func TestGetReference_WithRegistry(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "")
	require.NoError(t, err)
	require.Equal(t, "myhost", ociRegistry.GetReference())
}

func TestGetReference_WithRegistry_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithRegistry_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithRegistry_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "https://myhost", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithRegistry_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
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

func TestGetReference_WithRegistry_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path/", "", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithRegistry_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path", "", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithRegistry_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost/existing-path/", "", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myhost/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_WithScheme_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("https://myregistry.io")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_WithScheme_ContainingSubPath_AndAdditionalSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("http://myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_TrailingSlash_AndSubPath(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("myregistry.io/existing-path")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestGetReference_WithCredSecret_TrailingSlash_AndSubPath_LeadingSlash(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)

	dockerConfigJson, err := getDockerConfigJsonWithValidEntry("myregistry.io/existing-path/")
	require.NoError(t, err)
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": dockerConfigJson},
	}
	mockSecretGettr.On("Get", mock.Anything, "mysecret").Return(secret, nil)

	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "", "mysecret", "/kyma-modules")
	require.NoError(t, err)
	require.Equal(t, "myregistry.io/existing-path/kyma-modules", ociRegistry.GetReference())
}

func TestIsInsecure_WithHttpScheme(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "http://myhost", "", "")
	require.NoError(t, err)
	require.True(t, ociRegistry.IsInsecure())
}

func TestIsInsecure_WithHttpsScheme(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "https://myhost", "", "")
	require.NoError(t, err)
	require.False(t, ociRegistry.IsInsecure())
}

func TestIsInsecure_WithoutScheme(t *testing.T) {
	mockSecretGettr := new(mockSecretGetter)
	ociRegistry, err := setup.NewOCIRegistry(t.Context(), mockSecretGettr, "myhost", "", "")
	require.NoError(t, err)
	require.False(t, ociRegistry.IsInsecure())
}

func getDockerConfigJsonWithValidEntry(hostName string) ([]byte, error) {
	dockerConfig := map[string]any{
		"auths": map[string]any{
			hostName: map[string]string{"auth": "dXNlcjpwYXNz"},
		},
	}
	jsonDockerConfig, err := json.Marshal(dockerConfig)
	return jsonDockerConfig, err
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
