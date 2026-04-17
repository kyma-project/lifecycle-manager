package watch

import (
	"sigs.k8s.io/controller-runtime/pkg/handler"

	kymarepo "github.com/kyma-project/lifecycle-manager/internal/repository/kyma"
	"github.com/kyma-project/lifecycle-manager/internal/watch"
)

func ComposeTemplateChangeHandlerMapFunc(kymaRepo *kymarepo.Repository) handler.MapFunc {
	return watch.NewTemplateChangeHandler(kymaRepo).Watch()
}
