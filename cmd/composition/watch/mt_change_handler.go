package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/handler"

	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	mtrepo "github.com/kyma-project/lifecycle-manager/internal/repository/moduletemplate"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

func ComposeTemplateChangeHandlerMapFunc(mtRepo *mtrepo.Repository, kymaRepo *kymarepo.Repository) handler.MapFunc {
	return watch.NewTemplateChangeHandler(mtRepo, kymaRepo).Watch()
}
