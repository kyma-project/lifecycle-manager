package api

import (
	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

func AddToScheme(scheme *runtime.Scheme) error {
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := v1beta1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := scheme.SetVersionPriority(v1beta1.GroupVersion, v1alpha1.GroupVersion); err != nil {
		return err
	}
	if err := scheme.AddConversionFunc(&v1alpha1.Manifest{}, &v1beta1.Manifest{},
		func(a, b interface{}, scope conversion.Scope) error {
			return a.(*v1alpha1.Manifest).ConvertTo(b.(*v1beta1.Manifest))
		}); err != nil {
		return err
	}
	if err := scheme.AddConversionFunc(&v1alpha1.Kyma{}, &v1beta1.Kyma{},
		func(a, b interface{}, scope conversion.Scope) error {
			return a.(*v1alpha1.Kyma).ConvertTo(b.(*v1beta1.Kyma))
		}); err != nil {
		return err
	}
	if err := scheme.AddConversionFunc(&v1alpha1.ModuleTemplate{}, &v1beta1.ModuleTemplate{},
		func(a, b interface{}, scope conversion.Scope) error {
			return a.(*v1alpha1.ModuleTemplate).ConvertTo(b.(*v1beta1.ModuleTemplate))
		}); err != nil {
		return err
	}
	return scheme.AddConversionFunc(&v1alpha1.Watcher{}, &v1beta1.Watcher{},
		func(a, b interface{}, scope conversion.Scope) error {
			return a.(*v1alpha1.Watcher).ConvertTo(b.(*v1beta1.Watcher))
		})
}
