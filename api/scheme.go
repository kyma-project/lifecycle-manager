package api

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func AddToScheme(scheme *runtime.Scheme) error {
	if err := v1beta1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := v1beta2.AddToScheme(scheme); err != nil {
		return err
	}
	return scheme.SetVersionPriority(v1beta2.GroupVersion, v1beta1.GroupVersion)
}
