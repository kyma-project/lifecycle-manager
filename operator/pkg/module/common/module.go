package common

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	manifestV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
)

type (
	Modules map[string]*Module
	Module  struct {
		Name             string
		Template         *v1alpha1.ModuleTemplate
		TemplateOutdated bool
		*manifestV1alpha1.Manifest
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
	lbls := m.GetLabels()
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

	m.SetLabels(lbls)
}

func (m *Module) StateMismatchedWithTemplateInfo(info *v1alpha1.TemplateInfo) bool {
	return info.Generation != m.Template.GetGeneration() ||
		info.Channel != m.Template.Spec.Channel
}

// UpdateStatusAndReferencesFromUnstructured updates the module with necessary information (status, ownerReference) from
// current deployed resource (from Unstructured).
func (m *Module) UpdateStatusAndReferencesFromUnstructured(unstructured *manifestV1alpha1.Manifest) {
	m.Status = unstructured.Status
	m.SetResourceVersion(unstructured.GetResourceVersion())
	m.SetOwnerReferences(unstructured.GetOwnerReferences())
}

func (m *Module) ContainsExpectedOwnerReference(ownerName string) bool {
	if m.GetOwnerReferences() == nil {
		return false
	}
	for _, owner := range m.GetOwnerReferences() {
		if owner.Name == ownerName {
			return true
		}
	}
	return false
}

func NewFromModule(module *Module) *manifestV1alpha1.Manifest {
	fromServer := manifestV1alpha1.Manifest{}
	fromServer.SetGroupVersionKind(module.GroupVersionKind())
	fromServer.SetNamespace(module.GetNamespace())
	fromServer.SetName(module.GetName())
	return &fromServer
}

func CreateModuleName(moduleName, kymaName string) string {
	return fmt.Sprintf("%s-%s", kymaName, moduleName)
}
