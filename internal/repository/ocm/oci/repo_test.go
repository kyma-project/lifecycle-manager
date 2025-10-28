package oci_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/repository/ocm/oci"
)

func TestNewRepository(t *testing.T) {
	t.Run("should fail with nil keychain", func(t *testing.T) {
		// when
		repo, err := oci.NewRepository(nil, true)

		// then
		require.ErrorIs(t, err, oci.ErrKeyChainNotNil)
		require.Nil(t, repo)
	})
}

func TestGetConfig(t *testing.T) {
	t.Run("should fetch config file successfully", func(t *testing.T) {
		// given
		configCalled := false
		configContent := []byte("config content")
		receivedOptions := []crane.Option{}
		configFunc := func(ref string, opts ...crane.Option) ([]byte, error) {
			configCalled = true
			receivedOptions = opts
			return configContent, nil
		}

		kclStub := &kclStub{}
		repo, err := oci.NewRepository(kclStub, true, oci.WithConfigFunction(configFunc))
		require.NoError(t, err)

		ctx := t.Context()

		// when
		configBytes, err := repo.Config(ctx, "test-ref")

		// then
		require.NoError(t, err)
		require.True(t, configCalled, "expected config function to be called")
		require.Equal(t, []byte("config content"), configBytes)
		require.Equal(t, ctx, kclStub.ctx)
		require.Len(t, receivedOptions, 3)
	})

	t.Run("should return an error from config function", func(t *testing.T) {
		// given
		kclStub := &kclErrorStub{}

		repo, err := oci.NewRepository(kclStub, true)
		require.NoError(t, err)

		// when
		_, err = repo.Config(t.Context(), "test-ref")

		// then
		require.ErrorIs(t, err, errKeyChain)
	})
}

func TestPullLayer(t *testing.T) {
	t.Run("should pull layer successfully", func(t *testing.T) {
		// given
		pullCalled := false
		expectedLayer := static.NewLayer([]byte("layer content"), types.OCILayer)
		pullFunc := func(ref string, opts ...crane.Option) (containerregistryv1.Layer, error) {
			pullCalled = true
			return expectedLayer, nil
		}

		kclStub := &kclStub{}
		repo, err := oci.NewRepository(kclStub, true, oci.WithPullLayerFunction(pullFunc))
		require.NoError(t, err)

		ctx := t.Context()

		// when
		layer, err := repo.PullLayer(ctx, "test-ref")

		// then
		require.NoError(t, err)
		require.True(t, pullCalled, "expected pull function to be called")
		layerContent, err := layer.Uncompressed()
		require.NoError(t, err)
		expectedContent, err := expectedLayer.Uncompressed()
		require.NoError(t, err)
		require.Equal(t, expectedContent, layerContent)
		require.Equal(t, ctx, kclStub.ctx)
	})

	t.Run("should return an error from pull function", func(t *testing.T) {
		// given
		kclStub := &kclErrorStub{}

		repo, err := oci.NewRepository(kclStub, true)
		require.NoError(t, err)

		// when
		_, err = repo.PullLayer(t.Context(), "test-ref")

		// then
		require.ErrorIs(t, err, errKeyChain)
	})
}

var errKeyChain = errors.New("keychain error")

type kclErrorStub struct{}

func (e *kclErrorStub) Get(ctx context.Context) (authn.Keychain, error) {
	return nil, errKeyChain
}

type kclStub struct {
	ctx context.Context //nolint:containedctx //used to verify that the mock is called with the correct context
}

func (m *kclStub) Get(ctx context.Context) (authn.Keychain, error) {
	m.ctx = ctx
	return nil, nil
}
