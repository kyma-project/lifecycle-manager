package internal_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/controller-runtime/pkg/cache"

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
			14,
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
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := internal.ParseManifestToObjects(filepath.Join(testSamplesDir, testCase.manifestFile))
			if !testCase.wantErr(t, err,
				fmt.Sprintf("ParseManifestToObjects(%v)", testCase.manifestFile)) {
				return
			}
			assert.Lenf(t, got.Items, testCase.expectedResources,
				fmt.Sprintf("ParseManifestToObjects(%v)", testCase.manifestFile))
		})
	}
}

func Test_GetResourceNamespacesConfig(t *testing.T) {
	type args struct {
		resource      string
		namespacesMap map[string][]string
	}
	tests := []struct {
		name string
		args args
		want map[string]cache.Config
	}{
		{
			name: "Empty namespacesMap",
			args: args{
				resource:      "Kyma",
				namespacesMap: map[string][]string{},
			},
			want: map[string]cache.Config{},
		},
		{
			name: "NamespacesMap with values",
			args: args{
				resource: "Kyma",
				namespacesMap: map[string][]string{
					"Kyma":     {"kyma-system", "kcp-system"},
					"Manifest": {"kcp-system"},
				},
			},
			want: map[string]cache.Config{
				"kyma-system": {},
				"kcp-system":  {},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, internal.GetResourceNamespacesConfig(tt.args.resource, tt.args.namespacesMap),
				"GetResourceNamespacesConfig(%v, %v)", tt.args.resource, tt.args.namespacesMap)
		})
	}
}

func TestParseCacheNamespaces(t *testing.T) {
	tests := []struct {
		name       string
		namespaces string
		want       map[string][]string
	}{
		{
			name:       "Empty namespaces",
			namespaces: "",
			want:       map[string][]string{},
		},
		{
			name:       "Namespaces with values",
			namespaces: "Kyma:kyma-system,kcp-system;Manifest:kcp-system",
			want: map[string][]string{
				"Kyma":     {"kyma-system", "kcp-system"},
				"Manifest": {"kcp-system"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, internal.ParseCacheNamespaces(tt.namespaces), "ParseCacheNamespaces(%v)",
				tt.namespaces)
		})
	}
}
