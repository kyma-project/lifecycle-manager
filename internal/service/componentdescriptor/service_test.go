package componentdescriptor_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"testing"

	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ocm.software/ocm/api/ocm/compdesc"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/internal/service/componentdescriptor"
)

const (
	testComponentName            = "kyma-project.io/test-component"
	testComponentVersion         = "0.1.2"
	componentDescriptorMediaType = "application/vnd.ocm.software.component-descriptor.v2+yaml+tar"
	testHostRef                  = "k3d-kcp-registry.localhost:5000"
	testDigest                   = "sha256:4e51d8f80b88bdbd208e6e22314376a0d5212026bf3054f8ef79d43250e5182b"
)

func TestNewService(t *testing.T) {
	t.Run("should return error when ociRepository is nil", func(t *testing.T) {
		// when
		svc, err := componentdescriptor.NewService(nil)

		// then
		require.Nil(t, svc)
		require.ErrorIs(t, err, componentdescriptor.ErrInvalidArg)
	})
}

func TestGetComponentDescriptor(t *testing.T) {
	compdesc.RegisterScheme(&compdescv2.DescriptorVersion{})

	t.Run("should return component descriptor", func(t *testing.T) {
		// given
		cd := compdesc.New(testComponentName, testComponentVersion)
		cdBytes, err := compdesc.Encode(cd)
		require.NoError(t, err)
		tarBytes := wrapAsTar(t, componentdescriptor.ComponentDescriptorFileName, cdBytes)
		mockLayer := static.NewLayer(tarBytes, componentDescriptorMediaType)

		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerResult: mockLayer,
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)

		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)

		// then
		require.NoError(t, err)
		assert.Equal(t, testComponentName, result.GetName())
		assert.Equal(t, testComponentVersion, result.GetVersion())
		assert.Equal(t, testComponentVersion, repo.getConfigTag)
		assert.Equal(t, testComponentName, repo.getConfigName)
		assert.Equal(t, testComponentVersion, repo.pullLayerTag)
		assert.Equal(t, testComponentName, repo.pullLayerName)
		assert.Equal(t, testDigest, repo.pullLayerDigest)
	})

	t.Run("should fail when reading config object returns an error", func(t *testing.T) {
		// given
		repo := mockOCIRepository{
			getConfigError: errors.New("getConfigError"), // simulate error
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)
		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)
		// then
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(),
			"failed to get config file for ocm artifact with name=\""+
				testComponentName+
				"\" and version=\""+
				testComponentVersion)
		assert.Contains(t, err.Error(), "getConfigError")
	})

	t.Run("should fail when config object is not valid json", func(t *testing.T) {
		// given
		repo := mockOCIRepository{
			getConfigResult: []byte("...invalid json..."), // simulate error
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)
		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)
		// then
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(),
			"failed to unmarshal config data into ComponentDescriptorConfig for ocm artifact with name=\""+
				testComponentName+
				"\" and version=\""+
				testComponentVersion)
		assert.Contains(t, err.Error(), "invalid character")
	})

	t.Run("should fail when config object has null layer", func(t *testing.T) {
		repo := mockOCIRepository{
			getConfigResult: []byte(invalidConfigNullLayer),
			pullLayerResult: nil,
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)
		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)
		// then
		require.Error(t, err)
		assert.Nil(t, result)
		require.ErrorIs(t, err, componentdescriptor.ErrLayerNil)
		assert.Contains(t, err.Error(), "ComponentDescriptorLayer is nil in ComponentDescriptorConfig")
	})

	t.Run("should fail when config object has no digest", func(t *testing.T) {
		repo := mockOCIRepository{
			getConfigResult: []byte(invalidConfigNoDigest),
			pullLayerResult: nil,
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)
		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)
		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)
		// then
		assert.Nil(t, result)
		require.Error(t, err)
		require.ErrorIs(t, err, componentdescriptor.ErrLayerDigestEmpty)
		assert.Contains(t, err.Error(), "ComponentDescriptorLayer.Digest is empty in ComponentDescriptorConfig")
	})

	t.Run("should fail when pulling layer returns an error", func(t *testing.T) {
		// given
		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerError:  errors.New("pullLayerError"), // simulate error
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)
		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)
		// then
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(),
			"failed to pull layer for ocm artifact with name=\""+
				testComponentName+
				"\", version=\""+testComponentVersion+
				"\" and digest=\""+testDigest+"\"")
		assert.Contains(t, err.Error(), "pullLayerError")
	})

	t.Run("should fail when tar archive doesn't contain expected file", func(t *testing.T) {
		cd := compdesc.New(testComponentName, testComponentVersion)
		compdesc.RegisterScheme(&compdescv2.DescriptorVersion{})
		cdBytes, err := compdesc.Encode(cd)
		require.NoError(t, err)
		tarBytes := wrapAsTar(t, "invalid-name", cdBytes)
		mockLayer := static.NewLayer(tarBytes, componentDescriptorMediaType)

		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerResult: mockLayer,
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)

		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmi)

		// then
		require.Error(t, err)
		assert.Nil(t, result)
		require.ErrorIs(t, err, componentdescriptor.ErrNotFoundInTar)
		assert.Contains(t, err.Error(), "not found in TAR archive")
		assert.Contains(t, err.Error(), "failed to extract data from TAR archive")
	})

	t.Run("should fail on component descriptor decoding error", func(t *testing.T) {
		// given
		cd := compdesc.New(testComponentName, testComponentVersion)
		compdesc.RegisterScheme(&compdescv2.DescriptorVersion{})
		cdBytes, err := compdesc.Encode(cd)
		require.NoError(t, err)
		// introduce an error
		cdBytes = bytes.ReplaceAll(cdBytes, []byte("[]"), []byte("!}"))
		tarBytes := wrapAsTar(t, componentdescriptor.ComponentDescriptorFileName, cdBytes)
		mockLayer := static.NewLayer(tarBytes, componentDescriptorMediaType)

		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerResult: mockLayer,
		}

		svc, err := componentdescriptor.NewService(&repo)
		require.NoError(t, err)

		ocmi, err := ocmidentity.New(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		_, err = svc.GetComponentDescriptor(t.Context(), *ocmi)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode component descriptor fetched from ocm artifact with name=\""+
			testComponentName+"\" and version=\""+testComponentVersion+"\"")
	})
}

type mockOCIRepository struct {
	getConfigName   string
	getConfigTag    string
	pullLayerName   string
	pullLayerTag    string
	pullLayerDigest string
	getConfigResult []byte
	getConfigError  error
	pullLayerResult containerregistryv1.Layer
	pullLayerError  error
}

func (m *mockOCIRepository) GetConfigFile(ctx context.Context, name, tag string) ([]byte, error) {
	m.getConfigName = name
	m.getConfigTag = tag
	if m.getConfigError != nil {
		return nil, m.getConfigError
	}
	return m.getConfigResult, nil
}

func (m *mockOCIRepository) PullLayer(ctx context.Context, name, tag, digest string) (containerregistryv1.Layer, error) {
	m.pullLayerName = name
	m.pullLayerTag = tag
	m.pullLayerDigest = digest
	if m.pullLayerError != nil {
		return nil, m.pullLayerError
	}
	return m.pullLayerResult, nil
}

const (
	testComponentDescriptorConfig = `{"componentDescriptorLayer":{"mediaType":"application/vnd.ocm.software.component-descriptor.v2+yaml+tar","digest":"sha256:4e51d8f80b88bdbd208e6e22314376a0d5212026bf3054f8ef79d43250e5182b","size":4608}}`
	invalidConfigNullLayer        = `{"componentDescriptorLayer":null}`
	invalidConfigNoDigest         = `{"componentDescriptorLayer":{"mediaType":"application/vnd.ocm.software.component-descriptor.v2+yaml+tar","size":4608}}`
)

func wrapAsTar(t *testing.T, fileName string, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	twriter := tar.NewWriter(&buf)
	err := twriter.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     fileName,
		Size:     int64(len(data)),
		Mode:     0o600,
	})
	require.NoError(t, err)
	_, err = twriter.Write(data)
	require.NoError(t, err)
	err = twriter.Close()
	require.NoError(t, err)
	return buf.Bytes()
}
