// Package render is the composition root for the manifest-render service.
package render

import (
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

// ComposeRenderService wires the manifest ResourceRenderService: the default
// transform chain, plus the optional SkrImagePullSecret transform when
// skrImagePullSecretName is non-empty.
func ComposeRenderService(
	parser declarativev2.CachedManifestParser,
	skrImagePullSecretName string,
) *render.Service {
	transforms := declarativev2.GetDefaultResourceTransforms()
	if skrImagePullSecretName != "" {
		transforms = append(transforms,
			declarativev2.CreateSkrImagePullSecretTransform(skrImagePullSecretName))
	}
	return render.NewService(parser, transforms)
}
