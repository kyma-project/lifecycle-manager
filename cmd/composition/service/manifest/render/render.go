// Package render is the composition root for the manifest-render service.
package render

import (
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

// ComposeRenderService wires the manifest ResourceRenderService: the default
// transform chain, plus the optional SkrImagePullSecret transform when
// skrImagePullSecretName is non-empty.
func ComposeRenderService(
	cachedParser *parser.CachedManifestParser,
	skrImagePullSecretName string,
) *render.Service {
	transforms := render.GetDefaultResourceTransforms()
	if skrImagePullSecretName != "" {
		transforms = append(transforms,
			render.CreateSkrImagePullSecretTransform(skrImagePullSecretName))
	}
	return render.NewService(cachedParser, transforms)
}
