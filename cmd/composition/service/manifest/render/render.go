// Package render is the composition root for the manifest-render service.
package render

import (
	"slices"

	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/manifest/render"
)

// ComposeRenderService wires the manifest ResourceRenderService: the default
// transform chain, plus optional transforms enabled by configuration.
//
// The SkrImagePullSecret transform is appended when skrImagePullSecretName is non-empty.
// The RestrictedDefaultModuleImagePullSecret transform is appended only when the
// deployer module is configured as a restricted default module — see issue #3345.
func ComposeRenderService(
	parser declarativev2.CachedManifestParser,
	skrImagePullSecretName string,
	secretRepo declarativev2.SecretRepository,
	restrictedDefaultModules []string,
) *render.Service {
	transforms := declarativev2.GetDefaultResourceTransforms()
	if skrImagePullSecretName != "" {
		transforms = append(transforms,
			declarativev2.CreateSkrImagePullSecretTransform(skrImagePullSecretName))
	}
	if slices.Contains(restrictedDefaultModules, declarativev2.RestrictedDefaultModuleDeployerName) {
		transforms = append(transforms,
			declarativev2.CreateRestrictedDefaultModuleImagePullSecretTransform(secretRepo))
	}
	return render.NewService(parser, transforms)
}
