package generator

import (
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
)

var (
	ErrModuleNeedsTemplateInfo            = errors.New("module needs a template info")
	ErrModuleNeedsTemplateErrorOrTemplate = errors.New("module needs either a TemplateError or a ModuleTemplate")
	ErrModuleNeedsManifest                = errors.New("module needs either manifest or template error")
)

type GenerateFromErrorFunc func(err error, moduleName, desiredChannel, fqdn string,
	status *v1beta2.ModuleStatus) (*v1beta2.ModuleStatus, error)

type ModuleStatusGenerator struct {
	generateFromErrorFunc GenerateFromErrorFunc
}

func NewModuleStatusGenerator(fromErrorGeneratorFunc GenerateFromErrorFunc) *ModuleStatusGenerator {
	return &ModuleStatusGenerator{
		generateFromErrorFunc: fromErrorGeneratorFunc,
	}
}

func (m *ModuleStatusGenerator) GenerateModuleStatus(module *modulecommon.Module,
	currentStatus *v1beta2.ModuleStatus,
) (*v1beta2.ModuleStatus, error) {
	// This nil pointer check is for defensive programming and should never occur in a production environment.
	if module.TemplateInfo == nil {
		return nil, ErrModuleNeedsTemplateInfo
	}

	if module.TemplateInfo.ModuleTemplate == nil && module.TemplateInfo.Err == nil {
		return nil, ErrModuleNeedsTemplateErrorOrTemplate
	}

	if module.TemplateInfo.Err != nil {
		return m.generateFromErrorFunc(module.TemplateInfo.Err, module.ModuleName, module.TemplateInfo.DesiredChannel,
			module.FQDN, currentStatus)
	}

	// This nil pointer check is for defensive programming and should never occur in a production environment.
	if module.Manifest == nil {
		return nil, ErrModuleNeedsManifest
	}

	manifest := module.Manifest

	manifestAPIVersion, manifestKind := manifest.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	templateAPIVersion, templateKind := module.TemplateInfo.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	moduleStatus := &v1beta2.ModuleStatus{
		Name:    module.ModuleName,
		FQDN:    module.FQDN,
		State:   manifest.Status.State,
		Channel: module.TemplateInfo.DesiredChannel,
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
				Name:       module.TemplateInfo.GetName(),
				Namespace:  module.TemplateInfo.GetNamespace(),
				Generation: module.TemplateInfo.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: templateKind, APIVersion: templateAPIVersion},
		},
	}

	if manifest.Spec.Resource != nil &&
		manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyCreateAndDelete {
		moduleCRAPIVersion, moduleCRKind := manifest.Spec.Resource.GetObjectKind().
			GroupVersionKind().
			ToAPIVersionAndKind()
		moduleStatus.Resource = &v1beta2.TrackingObject{
			PartialMeta: v1beta2.PartialMeta{
				Name:       manifest.Spec.Resource.GetName(),
				Namespace:  manifest.Spec.Resource.GetNamespace(),
				Generation: manifest.Spec.Resource.GetGeneration(),
			},
			TypeMeta: apimetav1.TypeMeta{Kind: moduleCRKind, APIVersion: moduleCRAPIVersion},
		}

		if module.TemplateInfo.Annotations[shared.IsClusterScopedAnnotation] == shared.EnableLabelValue {
			moduleStatus.Resource.Namespace = ""
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
