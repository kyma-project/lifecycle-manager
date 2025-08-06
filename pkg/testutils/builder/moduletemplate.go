package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	machineryaml "k8s.io/apimachinery/pkg/util/yaml"
	"ocm.software/ocm/api/ocm/compdesc"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/random"
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
				Kind:       string(shared.ModuleTemplateKind),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      random.Name(),
				Namespace: apimetav1.NamespaceDefault,
			},
			Spec: v1beta2.ModuleTemplateSpec{
				Data: data,
				Descriptor: machineryruntime.RawExtension{
					Object: &types.Descriptor{
						ComponentDescriptor: &compdesc.ComponentDescriptor{
							Metadata: compdesc.Metadata{
								ConfiguredVersion: compdescv2.SchemaVersion,
							},
						},
					},
				},
			},
		},
	}
}

func (m ModuleTemplateBuilder) WithName(name string) ModuleTemplateBuilder {
	m.moduleTemplate.Name = name
	return m
}

func (m ModuleTemplateBuilder) WithNamespace(namespace string) ModuleTemplateBuilder {
	m.moduleTemplate.Namespace = namespace
	return m
}

func (m ModuleTemplateBuilder) WithVersion(version string) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Version = version
	return m
}

func (m ModuleTemplateBuilder) WithModuleName(moduleName string) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.ModuleName = moduleName
	return m
}

func (m ModuleTemplateBuilder) WithGeneration(generation int) ModuleTemplateBuilder {
	m.moduleTemplate.Generation = int64(generation)
	return m
}

func (m ModuleTemplateBuilder) WithLabelModuleName(moduleName string) ModuleTemplateBuilder {
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

func (m ModuleTemplateBuilder) WithDescriptor(descriptor *types.Descriptor) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Descriptor.Object = descriptor
	return m
}

func (m ModuleTemplateBuilder) WithRawDescriptor(rawDescriptor []byte) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Descriptor.Raw = rawDescriptor
	return m
}

func (m ModuleTemplateBuilder) WithOCM(schemaVersion compdesc.SchemaVersion) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.Descriptor = ComponentDescriptorFactoryFromSchema(schemaVersion)
	return m
}

func (m ModuleTemplateBuilder) WithRequiresDowntime(value bool) ModuleTemplateBuilder {
	m.moduleTemplate.Spec.RequiresDowntime = value
	return m
}

func (m ModuleTemplateBuilder) WithInternal(value bool) ModuleTemplateBuilder {
	if m.moduleTemplate.Labels == nil {
		m.moduleTemplate.Labels = make(map[string]string)
	}

	if value {
		m.moduleTemplate.Labels[shared.InternalLabel] = shared.EnableLabelValue
	}

	return m
}

func (m ModuleTemplateBuilder) WithBeta(value bool) ModuleTemplateBuilder {
	if m.moduleTemplate.Labels == nil {
		m.moduleTemplate.Labels = make(map[string]string)
	}

	if value {
		m.moduleTemplate.Labels[shared.BetaLabel] = shared.EnableLabelValue
	}

	return m
}

func (m ModuleTemplateBuilder) Build() *v1beta2.ModuleTemplate {
	return m.moduleTemplate
}

func ComponentDescriptorFactoryFromSchema(schemaVersion compdesc.SchemaVersion) machineryruntime.RawExtension {
	var moduleTemplate v1beta2.ModuleTemplate
	switch schemaVersion {
	case compdescv2.SchemaVersion:
		fallthrough
	default:
		template := "v1beta2_template_operator_current_ocm.yaml"
		readComponentDescriptorFromYaml(template, &moduleTemplate)
	}
	return moduleTemplate.Spec.Descriptor
}

func ReadComponentDescriptorFromFile(template string, moduleTemplate *v1beta2.ModuleTemplate) {
	// needs to be encapsulated in an outside call to make the runtime.Caller(1) find the proper path
	readComponentDescriptorFromYaml(template, moduleTemplate)
}

func readComponentDescriptorFromYaml(template string, moduleTemplate *v1beta2.ModuleTemplate) {
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(
		filepath.Dir(filename), "../../../tests/integration/moduletemplate", template,
	)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		panic(fmt.Errorf("read module template: %w", err))
	}
	err = machineryaml.Unmarshal(moduleFile, &moduleTemplate)
	if err != nil {
		panic(fmt.Errorf("unmarshal module template: %w", err))
	}
}
