package common

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type (
	Modules []*Module
	Module  struct {
		ModuleName       string
		OCMComponentName string
		TemplateInfo     *templatelookup.ModuleTemplateInfo
		Manifest         *v1beta2.Manifest
		Enabled          bool
		IsUnmanaged      bool
	}
)

func (m *Module) Logger(base logr.Logger) logr.Logger {
	return base.WithValues(
		"componentName", m.OCMComponentName,
		"module", m.Manifest.GetName(),
		"channel", m.TemplateInfo.Spec.Channel, //nolint:staticcheck // legacy Channel field
		"templateGeneration", m.TemplateInfo.GetGeneration(),
	)
}

func (m *Module) ApplyDefaultMetaToManifest(kyma *v1beta2.Kyma) {
	lbls := m.Manifest.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[shared.KymaName] = kyma.Name
	templateLabels := m.TemplateInfo.GetLabels()
	if templateLabels != nil {
		lbls[shared.ControllerName] = m.TemplateInfo.GetLabels()[shared.ControllerName]
	}
	lbls[shared.ModuleName] = m.ModuleName

	if !m.TemplateInfo.IsMandatory() {
		lbls[shared.ChannelLabel] = m.TemplateInfo.DesiredChannel
	}

	lbls[shared.ManagedBy] = shared.OperatorName
	if m.TemplateInfo.Spec.Mandatory {
		lbls[shared.IsMandatoryModule] = shared.EnableLabelValue
	}
	m.Manifest.SetLabels(lbls)

	anns := m.Manifest.GetAnnotations()
	if anns == nil {
		anns = make(map[string]string)
	}
	anns[shared.FQDN] = m.OCMComponentName
	anns[shared.OCMComponentName] = m.OCMComponentName
	if m.IsUnmanaged {
		anns[shared.UnmanagedAnnotation] = shared.EnableLabelValue
	}
	m.Manifest.SetAnnotations(anns)
}

func (m *Module) ContainsExpectedOwnerReference(ownerName string) bool {
	if m.Manifest.GetOwnerReferences() == nil {
		return false
	}
	for _, owner := range m.Manifest.GetOwnerReferences() {
		if owner.Name == ownerName {
			return true
		}
	}
	return false
}

const maxModuleNameLength = validation.DNS1035LabelMaxLength

// CreateModuleName takes an OCM Component Name and a prefix and generates a human-readable unique interpretation of
// a name combination.
// e.g. kyma-project.io/module/some-module and default-id => "default-id-some-module-34180237"
// e.g. domain.com/some-module and default-id => "default-id-some-module-1238916".
func CreateModuleName(compName, prefix, moduleName string) string {
	lastSeparatorIdx := strings.LastIndexByte(compName, '/')
	var lastPart string
	if lastSeparatorIdx != -1 {
		lastPart = compName[lastSeparatorIdx+1:]
	} else {
		lastPart = compName
	}
	hash := fnv.New32()
	_, _ = hash.Write([]byte(compName + moduleName))
	hashed := hash.Sum32()
	name := fmt.Sprintf("%s-%s-%v", prefix, lastPart, hashed)
	if len(name) >= maxModuleNameLength {
		name = name[:maxModuleNameLength-1]
	}
	return name
}
