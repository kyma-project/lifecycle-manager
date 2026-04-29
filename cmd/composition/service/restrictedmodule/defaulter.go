package restrictedmodule

import "github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"

func ComposeDefaulter(restrictedDefaultModules []string,
	moduleReleaseMetaRepo restrictedmodule.ModuleReleaseMetaRepository,
	kymaRepo restrictedmodule.KymaRepository,
) *restrictedmodule.Defaulter {
	return restrictedmodule.NewDefaulter(
		restrictedDefaultModules,
		moduleReleaseMetaRepo,
		kymaRepo,
		restrictedmodule.RestrictedModuleMatch,
	)
}
