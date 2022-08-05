package parsed

import (
	"github.com/go-logr/logr"
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
	}
)

func (m *Module) Channel() v1alpha1.Channel {
	return m.Template.Spec.Channel
}

func (m *Module) Logger(base logr.Logger) logr.Logger {
	return base.WithValues(
		"module", m.Name,
		"channel", m.Channel(),
		"templateGeneration", m.Template.GetGeneration(),
	)
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

func (m *Module) StateMismatchedWithCondition(condition *v1alpha1.KymaCondition) bool {
	return condition.TemplateInfo.Generation != m.Template.GetGeneration() ||
		condition.TemplateInfo.Channel != m.Template.Spec.Channel
}

// UpdateModuleFromCluster update the module with necessary information (status, ownerReference) from
// current deployed resource.
func (m *Module) UpdateModuleFromCluster(unstructuredFromServer *unstructured.Unstructured) {
	m.Unstructured.Object["status"] = unstructuredFromServer.Object["status"]
	m.Unstructured.SetResourceVersion(unstructuredFromServer.GetResourceVersion())
	m.Unstructured.SetOwnerReferences(unstructuredFromServer.GetOwnerReferences())
}

func (m *Module) ContainsExpectedOwnerReference(ownerName string) bool {
	if m.Unstructured.GetOwnerReferences() == nil {
		return false
	}
	for _, owner := range m.Unstructured.GetOwnerReferences() {
		if owner.Name == ownerName {
			return true
		}
	}
	return false
}
