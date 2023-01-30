package common

import (
	"fmt"
	"hash/fnv"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
)

type (
	Modules []*Module
	Module  struct {
		ModuleName       string
		FQDN             string
		Version          string
		Template         *v1alpha1.ModuleTemplate
		TemplateOutdated bool
		client.Object
	}
)

func (m *Module) Logger(base logr.Logger) logr.Logger {
	return base.WithValues(
		"fqdn", m.FQDN,
		"module", m.GetName(),
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
		(moduleStatus.Template.Generation != m.Template.GetGeneration() ||
			moduleStatus.Channel != m.Template.Spec.Channel)
	return templateStatusMismatch || moduleStatus.Manifest.GetGeneration() != m.GetGeneration()
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
