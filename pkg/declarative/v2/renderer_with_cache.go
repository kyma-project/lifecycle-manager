package v2

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/kyma-project/lifecycle-manager/internal"
	"github.com/kyma-project/lifecycle-manager/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	manifest = "manifest"
)

func WrapWithRendererCache(
	renderer Renderer,
	spec *Spec,
	options *Options,
) Renderer {
	if options.ManifestCache == NoManifestCache {
		return renderer
	}

	return &RendererWithCache{
		Renderer:      renderer,
		recorder:      options.EventRecorder,
		manifestCache: newManifestCache(string(options.ManifestCache), spec),
	}
}

type RendererWithCache struct {
	Renderer
	recorder record.EventRecorder
	*manifestCache
}

func (k *RendererWithCache) Render(ctx context.Context, obj Object) ([]byte, error) {
	logger := log.FromContext(ctx, "hash", k.hash, "Path", k.manifestCache.String())
	status := obj.GetStatus()

	if err := k.Clean(); err != nil {
		err := fmt.Errorf("cleaning cache failed: %w", err)
		k.recorder.Event(obj, "Warning", "ManifestCacheCleanup", err.Error())
		obj.SetStatus(status.WithState(StateError).WithErr(err))
		return nil, err
	}

	cacheFile := k.ReadYAML()

	if cacheFile.GetRawError() != nil {
		renderStart := time.Now()
		logger.Info("no cached manifest, rendering again")
		manifest, err := k.Renderer.Render(ctx, obj)
		if err != nil {
			k.recorder.Event(obj, "Warning", "RenderNonCached", err.Error())
			obj.SetStatus(status.WithState(StateError).WithErr(err))
			return nil, fmt.Errorf("rendering new manifest failed: %w", err)
		}
		logger.Info("rendering finished", "time", time.Since(renderStart))
		if err := internal.WriteToFile(k.manifestCache.String(), manifest); err != nil {
			k.recorder.Event(obj, "Warning", "ManifestCacheWrite", err.Error())
			obj.SetStatus(status.WithState(StateError).WithErr(err))
			return nil, err
		}
		return manifest, nil
	}

	logger.V(internal.DebugLogLevel).Info("reuse manifest from cache")

	return []byte(cacheFile.GetContent()), nil
}

type manifestCache struct {
	root string
	file string
	hash string
}

func newManifestCache(baseDir string, spec *Spec) *manifestCache {
	root := filepath.Join(baseDir, manifest, spec.Path)
	file := filepath.Join(root, spec.ManifestName)
	hashedValues, _ := internal.CalculateHash(spec.Values)
	hash := fmt.Sprintf("%v", hashedValues)
	file = fmt.Sprintf("%s-%s-%s.yaml", file, spec.Mode, hash)

	return &manifestCache{
		root: root,
		file: file,
		hash: fmt.Sprintf("%v", hashedValues),
	}
}

func (c *manifestCache) String() string {
	return c.file
}

func (c *manifestCache) Clean() error {
	removeAllOld := func(path string, info fs.FileInfo, err error) error {
		if info == nil || info.IsDir() {
			return nil
		}
		oldFile := filepath.Join(c.root, info.Name())
		if oldFile != c.file {
			return os.Remove(oldFile)
		}
		return nil
	}
	return filepath.Walk(c.root, removeAllOld)
}

func (c *manifestCache) ReadYAML() *types.ParsedFile {
	return types.NewParsedFile(internal.GetStringifiedYamlFromFilePath(c.String()))
}
