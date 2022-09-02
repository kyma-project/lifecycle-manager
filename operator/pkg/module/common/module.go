package common

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
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

	lbls[v1alpha1.ModuleName] = moduleName

	templateLabels := m.Template.GetLabels()
	if templateLabels != nil {
		lbls[v1alpha1.ControllerName] = m.Template.GetLabels()[v1alpha1.ControllerName]
	}
	lbls[v1alpha1.ChannelLabel] = string(m.Template.Spec.Channel)

	m.Unstructured.SetLabels(lbls)
}

func (m *Module) StateMismatchedWithTemplateInfo(info *v1alpha1.TemplateInfo) bool {
	return info.Generation != m.Template.GetGeneration() ||
		info.Channel != m.Template.Spec.Channel
}

// UpdateStatusAndReferencesFromUnstructured update the module with necessary information (status, ownerReference) from
// current deployed resource.
func (m *Module) UpdateStatusAndReferencesFromUnstructured(unstructured *unstructured.Unstructured) {
	m.Unstructured.Object["status"] = unstructured.Object["status"]
	m.Unstructured.SetResourceVersion(unstructured.GetResourceVersion())
	m.Unstructured.SetOwnerReferences(unstructured.GetOwnerReferences())
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

func NewUnstructuredFromModule(module *Module) *unstructured.Unstructured {
	unstructuredFromServer := unstructured.Unstructured{}
	unstructuredFromServer.SetGroupVersionKind(module.Unstructured.GroupVersionKind())
	unstructuredFromServer.SetNamespace(module.Unstructured.GetNamespace())
	unstructuredFromServer.SetName(module.Unstructured.GetName())
	return &unstructuredFromServer
}

func CreateModuleName(moduleName string, kymaName string) string {
	return moduleName + kymaName
}
