package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/ocm.software/v3alpha1"
	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ModuleTemplateBuilder struct {
	moduleTemplate *v1beta2.ModuleTemplate
}

func NewModuleTemplateBuilder() ModuleTemplateBuilder {
	data := NewModuleCRBuilder().Build()
	return ModuleTemplateBuilder{
		moduleTemplate: &v1beta2.ModuleTemplate{
			TypeMeta: apimetav1.TypeMeta{
				APIVersion: v1beta2.GroupVersion.String(),
				Kind:       string(shared.KymaKind),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      RandomName(),
				Namespace: apimetav1.NamespaceDefault,
			},
			Spec: v1beta2.ModuleTemplateSpec{
				Data: data,
			},
		},
	}
}

func (m ModuleTemplateBuilder) WithName(name string) ModuleTemplateBuilder {
	m.moduleTemplate.ObjectMeta.Name = name
	return m
}

func (m ModuleTemplateBuilder) WithModuleName(moduleName string) ModuleTemplateBuilder {
	if m.moduleTemplate.Labels == nil {
		m.moduleTemplate.Labels = make(map[string]string)
	}
	m.moduleTemplate.Labels[shared.ModuleName] = moduleName
	return m
}

func (m ModuleTemplateBuilder) WithChannel(channel string) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Channel = channel
	return m
}

func (m ModuleTemplateBuilder) WithMandatory(mandatory bool) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Mandatory = mandatory
	return m
}

func (m ModuleTemplateBuilder) WithAnnotation(key string, value string) ModuleTemplateBuilder {
	if m.moduleTemplate.Annotations == nil {
		m.moduleTemplate.Annotations = map[string]string{}
	}
	m.moduleTemplate.Annotations[key] = value
	return m
}

func (m ModuleTemplateBuilder) WithLabel(key string, value string) ModuleTemplateBuilder {
	if m.moduleTemplate.Labels == nil {
		m.moduleTemplate.Labels = map[string]string{}
	}
	m.moduleTemplate.Labels[key] = value
	return m
}

func (m ModuleTemplateBuilder) WithModuleCR(data *unstructured.Unstructured) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Data = data
	return m
}

func (m ModuleTemplateBuilder) WithOCM(schemaVersion compdesc.SchemaVersion) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Descriptor = ComponentDescriptorFactoryFromSchema(schemaVersion)
	return m
}

func (m ModuleTemplateBuilder) WithOCMPrivateRepo() ModuleTemplateBuilder {
	if m.moduleTemplate.Labels == nil {
		m.moduleTemplate.Labels = make(map[string]string)
	}
	var moduleTemplate v1beta2.ModuleTemplate
	template := "operator_v1beta2_moduletemplate_kcp-module-cred-label.yaml"
	readComponentDescriptorFromYaml(template, &moduleTemplate)
	m.moduleTemplate.Spec.Descriptor = moduleTemplate.Spec.Descriptor
	return m
}

func (m ModuleTemplateBuilder) Build() *v1beta2.ModuleTemplate {
	return m.moduleTemplate
}

func ComponentDescriptorFactoryFromSchema(schemaVersion compdesc.SchemaVersion) machineryruntime.RawExtension {
	var moduleTemplate v1beta2.ModuleTemplate
	switch schemaVersion {
	case compdescv2.SchemaVersion:
		template := "operator_v1beta2_moduletemplate_kcp-module.yaml"
		readComponentDescriptorFromYaml(template, &moduleTemplate)
	case v3alpha1.GroupVersion:
		fallthrough
	default:
		template := "operator_v1beta2_moduletemplate_ocm.software.v3alpha1.yaml"
		readComponentDescriptorFromYaml(template, &moduleTemplate)
	}
	return moduleTemplate.Spec.Descriptor
}

func readComponentDescriptorFromYaml(template string, moduleTemplate *v1beta2.ModuleTemplate) {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(
		filepath.Dir(filename), "../../../config/samples/component-integration-installed", template,
	)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		panic(fmt.Errorf("read module template: %w", err))
	}
	if err := machineryaml.Unmarshal(moduleFile, &moduleTemplate); err != nil {
		panic(fmt.Errorf("unmarshal module template: %w", err))
	}
}
