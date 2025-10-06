package internal_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kyma-project/lifecycle-manager/internal"
)

func TestParseManifestToObjects(t *testing.T) {
	t.Parallel()
	testSamplesDir := filepath.Join("..", "pkg", "test_samples", "oci")
	tests := []struct {
		name              string
		manifestFile      string
		expectedResources int
		wantErr           assert.ErrorAssertionFunc
	}{
		{
			"test btp operator manifest, expect no error",
			"btp-operator-manifest.yaml",
			7,
			assert.NoError,
		},
		{
			"test template operator manifest, expect no error",
			"template-operator-manifest.yaml",
			13,
			assert.NoError,
		},
		{
			"test any valid yaml, but is invalid manifest, expected no error",
			"valid-yaml.yaml",
			2,
			assert.NoError,
		},
		{
			"test invalid yaml, expected error",
			"invalid-yaml.yaml",
			0,
			assert.Error,
		},
		{
			"test manifest yaml contains duplicate resources, expected no error and duplicate get removed",
			"manifest-contains-duplicate-resources.yaml",
			1,
			assert.NoError,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := internal.ParseManifestToObjects(filepath.Join(testSamplesDir, testCase.manifestFile))
			if !testCase.wantErr(t, err,
				fmt.Sprintf("ParseManifestToObjects(%v)", testCase.manifestFile)) {
				return
			}
			assert.Lenf(t, got.Items, testCase.expectedResources, "ParseManifestToObjects(%v)", testCase.manifestFile)
		})
	}
}
