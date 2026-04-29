package restrictedmodule

import restrictedmodulesvc "github.com/kyma-project/lifecycle-manager/internal/service/restrictedmodule"

func ComposeDefaulter(restrictedDefaultModules []string,
	moduleReleaseMetaRepo restrictedmodulesvc.ModuleReleaseMetaRepository,
	kymaRepo restrictedmodulesvc.KymaRepository,
) *restrictedmodulesvc.Defaulter {
	return restrictedmodulesvc.NewDefaulter(
		restrictedDefaultModules,
		moduleReleaseMetaRepo,
		kymaRepo,
		restrictedmodulesvc.RestrictedModuleMatch,
	)
}
