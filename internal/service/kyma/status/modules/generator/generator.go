package generator

import (
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
)

var (
	ErrModuleNeedsTemplate = errors.New("module needs a template")
	ErrModuleNeedsManifest = errors.New("module needs either manifest or template error")
)

type GenerateFromErrorFunc func(err error, moduleName, desiredChannel, fqdn string, status *v1beta2.ModuleStatus) (v1beta2.ModuleStatus, error)

type ModuleStatusGenerator struct {
	generateFromErrorFunc GenerateFromErrorFunc
}

func NewModuleStatusGenerator(fromErrorGeneratorFunc GenerateFromErrorFunc) *ModuleStatusGenerator {
	return &ModuleStatusGenerator{
		generateFromErrorFunc: fromErrorGeneratorFunc,
	}
}

func (m *ModuleStatusGenerator) GenerateModuleStatus(module *common.Module, currentStatus *v1beta2.ModuleStatus) (v1beta2.ModuleStatus, error) {
	if module.Template == nil || module.Template.ModuleTemplate == nil {
		return v1beta2.ModuleStatus{}, ErrModuleNeedsTemplate
	}

	if module.Template.Err != nil {
		return m.generateFromErrorFunc(module.Template.Err, module.ModuleName, module.Template.DesiredChannel, module.FQDN, currentStatus)
	}

	if module.Manifest == nil {
		return v1beta2.ModuleStatus{}, ErrModuleNeedsManifest
	}

	manifest := module.Manifest

	manifestAPIVersion, manifestKind := manifest.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	templateAPIVersion, templateKind := module.Template.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	moduleStatus := v1beta2.ModuleStatus{
		Name:    module.ModuleName,
		FQDN:    module.FQDN,
		State:   manifest.Status.State,
		Channel: module.Template.DesiredChannel,
		Version: manifest.Spec.Version,
		Manifest: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       manifest.GetName(),
				Namespace:  manifest.GetNamespace(),
				Generation: manifest.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: manifestKind, APIVersion: manifestAPIVersion},
		},
		Template: &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       module.Template.GetName(),
				Namespace:  module.Template.GetNamespace(),
				Generation: module.Template.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
		},
	}

	if manifest.Spec.Resource != nil {
		moduleCRAPIVersion, moduleCRKind := manifest.Spec.Resource.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
		moduleStatus.Resource = &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       manifest.Spec.Resource.GetName(),
				Namespace:  manifest.Spec.Resource.GetNamespace(),
				Generation: manifest.Spec.Resource.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: moduleCRKind, APIVersion: moduleCRAPIVersion},
		}

		if module.Template.Annotations[shared.IsClusterScopedAnnotation] == shared.EnableLabelValue {
			moduleStatus.Resource.PartialMeta.Namespace = ""
		}
	}

	if module.IsUnmanaged {
		moduleStatus.State = shared.StateUnmanaged
		moduleStatus.Manifest = nil
		moduleStatus.Template = nil
		moduleStatus.Resource = nil
	}

	return moduleStatus, nil
}
