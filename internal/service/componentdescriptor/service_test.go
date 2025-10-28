package componentdescriptor_test

import (
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
	testComponentName             = "kyma-project.io/test-component"
	testComponentVersion          = "0.1.2"
	componentDescriptorMediaType  = "application/vnd.ocm.software.component-descriptor.v2+yaml+tar"
	testDigest                    = "sha256:4e51d8f80b88bdbd208e6e22314376a0d5212026bf3054f8ef79d43250e5182b"
	invalidConfigNullLayer        = `{"componentDescriptorLayer":null}`
	baseComponentDescriptorConfig = `{"componentDescriptorLayer":{` + `"mediaType":"` + componentDescriptorMediaType
	invalidConfigNoDigest         = baseComponentDescriptorConfig + `","size":4608}}`
	testComponentDescriptorConfig = baseComponentDescriptorConfig + `","digest":"` + testDigest + `","size":4608}}`
)

func TestNewService(t *testing.T) {
	t.Run("should return error when ociRepository is nil", func(t *testing.T) {
		// when
		svc, err := componentdescriptor.NewService(nil, nil)

		// then
		require.Nil(t, svc)
		require.ErrorIs(t, err, componentdescriptor.ErrInvalidArg)
		require.Contains(t, err.Error(), "ociRepository must not be nil")
	})

	t.Run("should return error when fileExtractor is nil", func(t *testing.T) {
		// given
		mockRepo := &mockOCIRepository{}
		// when
		svc, err := componentdescriptor.NewService(mockRepo, nil)

		// then
		require.Nil(t, svc)
		require.ErrorIs(t, err, componentdescriptor.ErrInvalidArg)
		require.Contains(t, err.Error(), "fileExtractor must not be nil")
	})
}

func TestGetComponentDescriptor(t *testing.T) {
	compdesc.RegisterScheme(&compdescv2.DescriptorVersion{})

	t.Run("should return component descriptor", func(t *testing.T) {
		// given
		mockLayer := static.NewLayer([]byte("dummy"), componentDescriptorMediaType)
		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerResult: mockLayer,
		}

		cd := compdesc.New(testComponentName, testComponentVersion)
		cdBytes, err := compdesc.Encode(cd)
		require.NoError(t, err)
		mockFileExtractor := &mockFileExtractor{mockData: cdBytes}

		subject, err := componentdescriptor.NewService(
			&repo,
			mockFileExtractor,
		)
		require.NoError(t, err)

		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := subject.GetComponentDescriptor(t.Context(), *ocmId)

		// then
		require.NoError(t, err)
		assert.Equal(t, testComponentName, result.GetName())
		assert.Equal(t, testComponentVersion, result.GetVersion())
		assert.Equal(t, mockLayer, mockFileExtractor.capturedLayer)
		assert.Equal(t, testComponentVersion, repo.capturedConfigTag)
		assert.Equal(t, testComponentName, repo.capturedConfigName)
		assert.Equal(t, testComponentVersion, repo.capturedPullLayerTag)
		assert.Equal(t, testComponentName, repo.capturedLayerName)
		assert.Equal(t, testDigest, repo.capturedPullLayerDigest)
	})

	t.Run("should fail when reading config object returns an error", func(t *testing.T) {
		// given
		repo := mockOCIRepository{
			getConfigError: errors.New("getConfigError"), // simulate error
		}

		svc, err := componentdescriptor.NewService(
			&repo,
			componentdescriptor.NewFileExtractor(componentdescriptor.NewTarExtractor()),
		)
		require.NoError(t, err)
		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmId)
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

		svc, err := componentdescriptor.NewService(&repo,
			componentdescriptor.NewFileExtractor(componentdescriptor.NewTarExtractor()),
		)
		require.NoError(t, err)
		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmId)
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

		svc, err := componentdescriptor.NewService(&repo,
			componentdescriptor.NewFileExtractor(componentdescriptor.NewTarExtractor()),
		)
		require.NoError(t, err)
		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmId)
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

		svc, err := componentdescriptor.NewService(&repo,
			componentdescriptor.NewFileExtractor(componentdescriptor.NewTarExtractor()),
		)
		require.NoError(t, err)
		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)
		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmId)
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

		svc, err := componentdescriptor.NewService(&repo,
			componentdescriptor.NewFileExtractor(componentdescriptor.NewTarExtractor()),
		)
		require.NoError(t, err)
		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := svc.GetComponentDescriptor(t.Context(), *ocmId)
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
		mockLayer := static.NewLayer([]byte("dummy"), componentDescriptorMediaType)

		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerResult: mockLayer,
		}

		expectedErr := errors.New("not found in TAR archive")
		mockFileExtractor := &mockFileExtractor{err: expectedErr}

		subject, err := componentdescriptor.NewService(
			&repo,
			mockFileExtractor,
		)
		require.NoError(t, err)

		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		result, err := subject.GetComponentDescriptor(t.Context(), *ocmId)

		// then
		require.Error(t, err)
		assert.Nil(t, result)
		require.ErrorIs(t, err, expectedErr)
		assert.Contains(t, err.Error(), "not found in TAR archive")
	})

	t.Run("should fail on component descriptor decoding error", func(t *testing.T) {
		// given
		mockLayer := static.NewLayer([]byte("dummy"), componentDescriptorMediaType)
		repo := mockOCIRepository{
			getConfigResult: []byte(testComponentDescriptorConfig),
			pullLayerResult: mockLayer,
		}

		cd := compdesc.New(testComponentName, testComponentVersion)
		cdBytes, err := compdesc.Encode(cd)
		require.NoError(t, err)
		// introduce an error
		brokenBytes := bytes.ReplaceAll(cdBytes, []byte("[]"), []byte("!}"))

		mockFileExtractor := &mockFileExtractor{mockData: brokenBytes}
		subject, err := componentdescriptor.NewService(
			&repo,
			mockFileExtractor,
		)
		require.NoError(t, err)

		ocmId, err := ocmidentity.NewComponentId(testComponentName, testComponentVersion)
		require.NoError(t, err)

		// when
		_, err = subject.GetComponentDescriptor(t.Context(), *ocmId)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get component descriptor for ocm artifact with name=\""+
			testComponentName+"\" and version=\""+testComponentVersion+"\": failed to decode component descriptor")
	})
}

type mockOCIRepository struct {
	capturedConfigName      string
	capturedConfigTag       string
	capturedLayerName       string
	capturedPullLayerTag    string
	capturedPullLayerDigest string
	getConfigResult         []byte
	getConfigError          error
	pullLayerResult         containerregistryv1.Layer
	pullLayerError          error
}

func (m *mockOCIRepository) GetConfig(ctx context.Context, name, tag string) ([]byte, error) {
	m.capturedConfigName = name
	m.capturedConfigTag = tag
	if m.getConfigError != nil {
		return nil, m.getConfigError
	}
	return m.getConfigResult, nil
}

func (m *mockOCIRepository) PullLayer(ctx context.Context, name, tag, digest string) (
	containerregistryv1.Layer, error,
) {
	m.capturedLayerName = name
	m.capturedPullLayerTag = tag
	m.capturedPullLayerDigest = digest
	if m.pullLayerError != nil {
		return nil, m.pullLayerError
	}
	return m.pullLayerResult, nil
}

type mockFileExtractor struct {
	mockData      []byte
	capturedLayer containerregistryv1.Layer // used to verify which layer was passed in
	err           error
}

func (m *mockFileExtractor) ExtractFileFromLayer(layer containerregistryv1.Layer, fileName string) ([]byte, error) {
	m.capturedLayer = layer
	if m.err != nil {
		return nil, m.err
	}
	return m.mockData, nil
}
