// Package render is the composition root for the manifest-render service.
package render

import (
	"slices"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

// ComposeRenderService wires the manifest ResourceRenderService: the default
// transform chain, plus optional transforms enabled by configuration.
//
// The SkrImagePullSecret transform is appended when skrImagePullSecretName is non-empty.
// The DeployerModuleImagePullSecret transform is appended only when the
// deployer module is configured as a restricted default module — see issue #3345.
func ComposeRenderService(
	cachedParser *parser.CachedManifestParser,
	skrImagePullSecretName string,
	secretRepo render.SecretRepository,
	restrictedDefaultModules []string,
) *render.Service {
	transforms := render.GetDefaultResourceTransforms()
	if skrImagePullSecretName != "" {
		transforms = append(transforms,
			render.CreateSkrImagePullSecretTransform(skrImagePullSecretName))
	}
	if slices.Contains(restrictedDefaultModules, render.DeployerModuleName) {
		transforms = append(transforms,
			render.CreateDeployerModuleImagePullSecretTransform(secretRepo))
	}
	return render.NewService(cachedParser, transforms)
}
