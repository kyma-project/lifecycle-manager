package api

import (
	"fmt"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	pkgapiv1beta1 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta1"
	pkgapiv1beta2 "github.com/kyma-project/lifecycle-manager/pkg/api/v1beta2"
)

func AddToScheme(scheme *machineryruntime.Scheme) error {
	if err := pkgapiv1beta1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add scheme on v1beta1 api: %w", err)
	}
	if err := pkgapiv1beta2.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add scheme on v1beta2 api: %w", err)
	}
	err := scheme.SetVersionPriority(v1beta2.GroupVersion, v1beta1.GroupVersion)
	if err != nil {
		return fmt.Errorf("failed to set which version priority: %w", err)
	}
	return nil
}
