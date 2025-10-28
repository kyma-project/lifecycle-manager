package ocm_test

import (
	"context"
	"errors"
	"testing"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/repository/ocm"
)

const (
	commonOCIArtifactRef = "europe-docker.pkg.dev/kyma-project/prod/component-descriptors/"
)

func TestNewRepository(t *testing.T) {
	tests := []struct {
		name      string
		hostPort  string
		expectErr bool
	}{
		{
			name:      "valid",
			hostPort:  "example.com:5000",
			expectErr: false,
		},
		{
			name:      "invalid with protocol in hostPort",
			hostPort:  "https://example.com:5000",
			expectErr: true,
		},
		{
			name:      "invalid with leading slash in hostPort",
			hostPort:  "/example.com:5000",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ocm.NewRepository(tt.hostPort, &ociRepoStub{})
			if (err != nil) != tt.expectErr {
				t.Errorf("NewRepository() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	t.Run("should fetch config file successfully", func(t *testing.T) {
		// given
		ociRepo := ociRepoStub{
			configResult: []byte("mock config data"),
		}
		repo, err := ocm.NewRepository("europe-docker.pkg.dev/kyma-project/prod", &ociRepo)
		require.NoError(t, err)
		// when
		configData, err := repo.GetConfig(t.Context(), "test-name", "test-version")
		// then
		require.NoError(t, err)
		assert.Equal(t, []byte("mock config data"), configData)
		assert.Equal(t, commonOCIArtifactRef+"test-name:test-version",
			ociRepo.configRefArg,
		)
	})

	t.Run("should return an error from craneClient.Config", func(t *testing.T) {
		// given
		repo, err := ocm.NewRepository(
			"europe-docker.pkg.dev/kyma-project/prod",
			&ociRepoStub{})
		require.NoError(t, err)
		// when
		_, err = repo.GetConfig(t.Context(), "test-name", "test-version")
		// then
		require.Error(t, err)
		require.ErrorIs(t, err, errMockConfig)
		assert.Contains(t, err.Error(), "failed to get config file for ref=")
		assert.Contains(t, err.Error(), commonOCIArtifactRef+
			"test-name:test-version",
		)
	})
}

func TestPullLayer(t *testing.T) {
	t.Run("should pull layer successfully", func(t *testing.T) {
		mockLayer := static.NewLayer([]byte("mock layer data"), "application/mock")
		ociRepo := ociRepoStub{
			pullResult: mockLayer,
		}

		repo, err := ocm.NewRepository(
			"europe-docker.pkg.dev/kyma-project/prod", &ociRepo)
		require.NoError(t, err)

		layer, err := repo.PullLayer(t.Context(),
			"test-name",
			"test-version",
			"sha256:abcdef1234567890")
		require.NoError(t, err)
		assert.Equal(t, mockLayer, layer)
		assert.Equal(t, commonOCIArtifactRef+"test-name:test-version@sha256:abcdef1234567890",
			ociRepo.pullRefArg,
		)
	})

	t.Run("should return an error from craneClient.PullLayer", func(t *testing.T) {
		repo, err := ocm.NewRepository(
			"europe-docker.pkg.dev/kyma-project/prod", &ociRepoStub{})
		require.NoError(t, err)
		_, err = repo.PullLayer(t.Context(), "test-name", "test-version", "sha256:abcdef1234567890")
		require.Error(t, err)
		require.ErrorIs(t, err, errMockPull)
		assert.Contains(t, err.Error(), "failed to pull layer for ref=")
		assert.Contains(t, err.Error(), commonOCIArtifactRef+"test-name:test-version@sha256:abcdef1234567890")
	})
}

var (
	errMockConfig = errors.New("mock config error")
	errMockPull   = errors.New("mock pull error")
)

type ociRepoStub struct {
	configRefArg string
	configResult []byte

	pullRefArg string
	pullResult containerregistryv1.Layer
}

func (m *ociRepoStub) Config(ctx context.Context, ref string) ([]byte, error) {
	m.configRefArg = ref
	if m.configResult == nil {
		return nil, errMockConfig
	}
	return m.configResult, nil
}

func (m *ociRepoStub) PullLayer(ctx context.Context, ref string) (containerregistryv1.Layer, error) {
	m.pullRefArg = ref
	if m.pullResult == nil {
		return nil, errMockPull
	}
	return m.pullResult, nil
}
