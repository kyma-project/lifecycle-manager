package parser_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/spec"
)

const sampleManifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: example
  namespace: default
data:
  key: value
`

func writeManifest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestCachedManifestParser_Parse_ParsesAndCachesResults(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, sampleManifest)
	p := parser.NewCachedManifestParser(time.Hour)

	first, err := p.Parse(&spec.Spec{ManifestName: "m", Path: path})
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	assert.Equal(t, "ConfigMap", first.Items[0].GetKind())

	// Mutate the returned copy and re-parse — the cache must hand back a
	// fresh deep copy each time.
	first.Items[0].SetName("mutated")

	second, err := p.Parse(&spec.Spec{ManifestName: "m", Path: path})
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	assert.Equal(t, "example", second.Items[0].GetName(),
		"Parse should return an independent deep copy on every call")
}

func TestCachedManifestParser_EvictCache_ForcesReparse(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, sampleManifest)
	p := parser.NewCachedManifestParser(time.Hour)
	sp := &spec.Spec{ManifestName: "m", Path: path}

	_, err := p.Parse(sp)
	require.NoError(t, err)

	// Overwrite the on-disk manifest after the cache is warm.
	require.NoError(t, os.WriteFile(path, []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: replaced
  namespace: default
`), 0o600))

	// Without eviction the cached entry is still served.
	cached, err := p.Parse(sp)
	require.NoError(t, err)
	require.Len(t, cached.Items, 1)
	assert.Equal(t, "example", cached.Items[0].GetName())

	p.EvictCache(sp)

	fresh, err := p.Parse(sp)
	require.NoError(t, err)
	assert.Equal(t, "replaced", fresh.Items[0].GetName())
}

func TestCachedManifestParser_Parse_ReturnsErrorOnMissingPath(t *testing.T) {
	t.Parallel()

	p := parser.NewCachedManifestParser(time.Hour)
	_, err := p.Parse(&spec.Spec{ManifestName: "m", Path: filepath.Join(t.TempDir(), "missing.yaml")})

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse manifest objects")
}
