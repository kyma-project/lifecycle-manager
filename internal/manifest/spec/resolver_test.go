package spec_test

import (
	"context"
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

const (
	mockLocalFileCachePath = "/some/local/path"
	testManifest           = `
apiVersion: operator.kyma-project.io/v1beta2
kind: Manifest
metadata:
  name: kyma-sample-template-operator-2235966007
  namespace: kcp-system
spec:
  install:
    name: raw-manifest
    source:
      name: kyma-project.io/module/template-operator
      ref: sha256:c49b23729d7f12e25a44bbc9c0fb226f998cb443802af4793b4faea79a9bac40
      repo: http://k3d-registry.localhost:5000/component-descriptors
      type: oci-ref
  remote: true
  resource:
    apiVersion: operator.kyma-project.io/v1alpha1
    kind: Sample
    metadata:
      name: sample-yaml
      namespace: kyma-system
    spec:
      resourceFilePath: ./module-data/yaml
  version: 1.0.0
`
)

func Test_GetSpec(t *testing.T) {
	t.Run("should return a Spec with the correct fields", func(t *testing.T) {
		// given
		specResolver := spec.NewResolver(&mockKeyChainLookup{}, &mockPathExtractor{})

		// when
		mft := v1beta2.Manifest{}
		err := yaml.Unmarshal([]byte(testManifest), &mft)
		require.NoError(t, err)

		actual, err := specResolver.GetSpec(t.Context(), &mft)
		require.NoError(t, err)

		// then
		expected := &declarativev2.Spec{
			ManifestName: mft.Spec.Install.Name,
			Path:         testPath(),
			OCIRef:       "sha256:c49b23729d7f12e25a44bbc9c0fb226f998cb443802af4793b4faea79a9bac40",
		}

		require.Equal(t, expected, actual)
	})

	t.Run("should return an error with incorrect render mode", func(t *testing.T) {
		// given
		specResolver := spec.NewResolver(&mockKeyChainLookup{}, &mockPathExtractor{})

		invalidManifest := strings.ReplaceAll(testManifest, "type: oci-ref", "type: invalid-ref")

		// when
		mft := v1beta2.Manifest{}
		err := yaml.Unmarshal([]byte(invalidManifest), &mft)
		require.NoError(t, err)

		_, err = specResolver.GetSpec(t.Context(), &mft)
		require.ErrorIs(t, err, spec.ErrRenderModeInvalid)
		require.ErrorContains(t, err, "could not determine render mode for")
	})

	t.Run("should return an error when keyChainLookup fails", func(t *testing.T) {
		// given
		errorKeychainLookup := &mockKeyChainLookup{mockError: errors.New("unexpected")}
		specResolver := spec.NewResolver(errorKeychainLookup, &mockPathExtractor{})

		// when
		mft := v1beta2.Manifest{}
		err := yaml.Unmarshal([]byte(testManifest), &mft)
		require.NoError(t, err)

		_, err = specResolver.GetSpec(t.Context(), &mft)
		require.ErrorContains(t, err, "failed to fetch keyChain: unexpected")
	})

	t.Run("should return an error when pathExtractor fails", func(t *testing.T) {
		// given
		errorPathExtractor := &mockPathExtractor{mockError: errors.New("unexpected")}
		specResolver := spec.NewResolver(&mockKeyChainLookup{}, errorPathExtractor)

		// when
		mft := v1beta2.Manifest{}
		err := yaml.Unmarshal([]byte(testManifest), &mft)
		require.NoError(t, err)

		_, err = specResolver.GetSpec(t.Context(), &mft)
		require.ErrorContains(t, err, "failed to extract raw manifest from layer digest: unexpected")
	})
}

type mockKeyChainLookup struct {
	mockError error
}

func (m *mockKeyChainLookup) Get(_ context.Context) (authn.Keychain, error) {
	return nil, m.mockError
}

type mockPathExtractor struct {
	mockError error
}

func (m *mockPathExtractor) GetPathFromRawManifest(
	_ context.Context,
	_ v1beta2.ImageSpec,
	_ authn.Keychain,
) (string, error) {
	if m.mockError != nil {
		return "", m.mockError
	}
	return testPath(), nil
}

func testPath() string {
	return path.Join(mockLocalFileCachePath, string(v1beta2.RawManifestLayer+".yaml"))
}
