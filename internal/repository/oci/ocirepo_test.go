package oci_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
	"github.com/kyma-project/lifecycle-manager/internal/repository/oci"
)

const (
	commonOCIArtifactRef = "europe-docker.pkg.dev/kyma-project/prod/component-descriptors/"
)

func TestNewRepository(t *testing.T) {
	tests := []struct {
		name      string
		hostPort  string
		insecure  bool
		kcl       spec.KeyChainLookup
		expectErr bool
	}{
		{
			name:      "valid with insecure",
			hostPort:  "example.com:5000",
			insecure:  true,
			kcl:       nil,
			expectErr: false,
		},
		{
			name:      "valid without insecure and with keychain",
			hostPort:  "example.com:5000",
			insecure:  false,
			kcl:       &mockKeyChainLookup{},
			expectErr: false,
		},
		{
			name:      "invalid: secure and without keychain",
			hostPort:  "example.com:5000",
			insecure:  false,
			kcl:       nil,
			expectErr: true,
		},
		{
			name:      "invalid with protocol in hostPort",
			hostPort:  "https://example.com:5000",
			insecure:  true,
			kcl:       nil,
			expectErr: true,
		},
		{
			name:      "invalid with leading slash in hostPort",
			hostPort:  "/example.com:5000",
			insecure:  true,
			kcl:       nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := oci.NewRepository(tt.kcl, tt.hostPort, tt.insecure, &oci.DefaultCraneWrapper{})
			if (err != nil) != tt.expectErr {
				t.Errorf("NewRepository() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestGetConfigFile(t *testing.T) {
	t.Run("should fetch config file successfully", func(t *testing.T) {
		// given
		mcc := mockCraneClient{
			configResult: []byte("mock config data"),
		}
		repo, err := oci.NewRepository(&mockKeyChainLookup{}, "europe-docker.pkg.dev/kyma-project/prod", true, &mcc)
		require.NoError(t, err)
		// when
		configData, err := repo.GetConfigFile(t.Context(), "test-name", "test-version")
		// then
		require.NoError(t, err)
		assert.Equal(t, []byte("mock config data"), configData)
		assert.Equal(t, commonOCIArtifactRef+"test-name:test-version",
			mcc.configRefArg,
		)

		opts := mcc.configOptsArg
		assert.Len(t, opts, 3) // one for context, one for keychain, one for insecure
	})

	t.Run("should return an error from KeyChain Lookup", func(t *testing.T) {
		// given
		repo, err := oci.NewRepository(&errorKeyChainLookup{}, "dummy", false, &oci.DefaultCraneWrapper{})
		require.NoError(t, err)
		// when
		_, err = repo.GetConfigFile(t.Context(), "test-name", "test-version")
		// then
		require.Error(t, err)
		require.ErrorIs(t, err, errKeyChain)
		assert.Contains(t, err.Error(), "failed to get keychain:")
	})

	t.Run("should use provided keychain lookup when secure", func(t *testing.T) {
		// given
		mcc := mockCraneClient{
			configResult: []byte("mock config data"),
		}
		mkcl := &mockKeyChainLookup{}
		repo, err := oci.NewRepository(mkcl, "europe-docker.pkg.dev/kyma-project/prod", false, &mcc)
		require.NoError(t, err)
		assert.Nil(t, mkcl.ctx)
		// when
		configData, err := repo.GetConfigFile(t.Context(), "test-name", "test-version")
		// then
		require.NoError(t, err)
		assert.Equal(t, []byte("mock config data"), configData)
		// ensure that the keychain lookup was called (and so used) with the given context:
		assert.Equal(t, t.Context(), mkcl.ctx)
	})

	t.Run("should return an error from craneClient.Config", func(t *testing.T) {
		// given
		repo, err := oci.NewRepository(&mockKeyChainLookup{},
			"europe-docker.pkg.dev/kyma-project/prod", true, &mockCraneClient{})
		require.NoError(t, err)
		// when
		_, err = repo.GetConfigFile(t.Context(), "test-name", "test-version")
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
		mcc := mockCraneClient{
			pullResult: mockLayer,
		}

		repo, err := oci.NewRepository(&mockKeyChainLookup{},
			"europe-docker.pkg.dev/kyma-project/prod", true, &mcc)
		require.NoError(t, err)

		layer, err := repo.PullLayer(t.Context(), "test-name", "test-version", "sha256:abcdef1234567890")
		require.NoError(t, err)
		assert.Equal(t, mockLayer, layer)
		assert.Equal(t, commonOCIArtifactRef+"test-name:test-version@sha256:abcdef1234567890",
			mcc.pullRefArg,
		)

		opts := mcc.pullOptsArg
		assert.Len(t, opts, 3) // one for context, one for keychain, one for insecure
	})

	t.Run("should return an error from KeyChain Lookup", func(t *testing.T) {
		repo, err := oci.NewRepository(&errorKeyChainLookup{},
			"europe-docker.pkg.dev/kyma-project/prod", false, &oci.DefaultCraneWrapper{})
		require.NoError(t, err)
		_, err = repo.PullLayer(t.Context(), "test-name", "test-version", "sha256:abcdef1234567890")
		require.Error(t, err)
		assert.ErrorIs(t, err, errKeyChain)
	})

	t.Run("should return an error from craneClient.PullLayer", func(t *testing.T) {
		repo, err := oci.NewRepository(&mockKeyChainLookup{},
			"europe-docker.pkg.dev/kyma-project/prod", true, &mockCraneClient{})
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

type mockCraneClient struct {
	configRefArg  string
	configOptsArg []crane.Option
	configResult  []byte

	pullRefArg  string
	pullOptsArg []crane.Option
	pullResult  containerregistryv1.Layer
}

func (m *mockCraneClient) Config(ref string, opts ...crane.Option) ([]byte, error) {
	m.configRefArg = ref
	m.configOptsArg = opts
	if m.configResult == nil {
		return nil, errMockConfig
	}
	return m.configResult, nil
}

func (m *mockCraneClient) PullLayer(ref string, opt ...crane.Option) (containerregistryv1.Layer, error) {
	m.pullRefArg = ref
	m.pullOptsArg = opt
	if m.pullResult == nil {
		return nil, errMockPull
	}
	return m.pullResult, nil
}

var errKeyChain = errors.New("keychain error")

type errorKeyChainLookup struct{}

func (e *errorKeyChainLookup) Get(ctx context.Context) (authn.Keychain, error) {
	return nil, errKeyChain
}

type mockKeyChainLookup struct {
	ctx context.Context //nolint:containedctx //used to verify that the mock is called with the correct context
}

func (m *mockKeyChainLookup) Get(ctx context.Context) (authn.Keychain, error) {
	m.ctx = ctx
	return nil, nil
}
