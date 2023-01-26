package common

import (
	"fmt"
	"hash/fnv"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	manifestV1alpha1 "github.com/kyma-project/module-manager/api/v1alpha1"
)

type (
	Modules []*Module
	Module  struct {
		For              string
		FQDN             string
		Version          string
		Template         *v1alpha1.ModuleTemplate
		TemplateOutdated bool
		*manifestV1alpha1.Manifest
	}
)

func (m *Module) Logger(base logr.Logger) logr.Logger {
	return base.WithValues(
		"fqdn", m.FQDN,
		"module", m.Name,
		"channel", m.Template.Spec.Channel,
		"templateGeneration", m.Template.GetGeneration(),
	)
}

func (m *Module) ApplyLabelsAndAnnotations(
	kyma *v1alpha1.Kyma,
) {
	lbls := m.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[v1alpha1.KymaName] = kyma.Name

	templateLabels := m.Template.GetLabels()
	if templateLabels != nil {
		lbls[v1alpha1.ControllerName] = m.Template.GetLabels()[v1alpha1.ControllerName]
	}
	lbls[v1alpha1.ChannelLabel] = m.Template.Spec.Channel

	m.SetLabels(lbls)

	anns := m.GetAnnotations()
	if anns == nil {
		anns = make(map[string]string)
	}
	anns[v1alpha1.FQDN] = m.FQDN
	m.SetAnnotations(anns)
}

func (m *Module) StateMismatchedWithModuleStatus(moduleStatus *v1alpha1.ModuleStatus) bool {
	templateStatusMismatch := m.TemplateOutdated &&
		(moduleStatus.TemplateInfo.Generation != m.Template.GetGeneration() ||
			moduleStatus.TemplateInfo.Channel != m.Template.Spec.Channel)
	return templateStatusMismatch || moduleStatus.Generation != m.GetGeneration()
}

// UpdateStatusAndReferencesFromUnstructured updates the module with necessary information (status, ownerReference) from
// current deployed resource (from Unstructured).
func (m *Module) UpdateStatusAndReferencesFromUnstructured(unstructured *manifestV1alpha1.Manifest) {
	m.Status = unstructured.Status
	m.SetResourceVersion(unstructured.GetResourceVersion())
	m.SetOwnerReferences(unstructured.GetOwnerReferences())
	m.SetGeneration(unstructured.GetGeneration())
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

const maxModuleNameLength = 253

func CreateModuleName(fqdn, kymaName string) string {
	hash := fnv.New32()
	_, _ = hash.Write([]byte(fqdn))
	hashedFQDN := hash.Sum32()
	name := fmt.Sprintf("%s-%v", kymaName, hashedFQDN)
	if len(name) >= maxModuleNameLength {
		name = name[:maxModuleNameLength-1]
	}
	return name
}
