package parsed

import (
	"github.com/imdario/mergo"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type (
	Modules map[string]*Module
	Module  struct {
		Name             string
		Template         *v1alpha1.ModuleTemplate
		TemplateOutdated bool
		*unstructured.Unstructured
		Settings unstructured.Unstructured
	}
)

func (m *Module) Channel() v1alpha1.Channel {
	return m.Template.Spec.Channel
}

func (m *Module) ApplyLabels(
	kyma *v1alpha1.Kyma,
	moduleName string,
) {
	lbls := m.Unstructured.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[v1alpha1.KymaName] = kyma.Name
	lbls[v1alpha1.ProfileLabel] = string(kyma.Spec.Profile)

	lbls[v1alpha1.ModuleName] = moduleName

	templateLabels := m.Template.GetLabels()
	if templateLabels != nil {
		lbls[v1alpha1.ControllerName] = m.Template.GetLabels()[v1alpha1.ControllerName]
	}
	lbls[v1alpha1.ChannelLabel] = string(m.Template.Spec.Channel)

	m.Unstructured.SetLabels(lbls)
}

func (m *Module) CopySettingsToUnstructured() error {
	overrideSpec := m.Settings.Object["spec"]
	if overrideSpec != nil {
		if err := mergo.Merge(m.Unstructured.Object["spec"], overrideSpec); err != nil {
			return err
		}
	}
	return nil
}
