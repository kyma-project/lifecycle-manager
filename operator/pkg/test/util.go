package test

import (
	"os"
	"path/filepath"
	"runtime"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

func ModuleTemplateFactory(module v1alpha1.Module, data unstructured.Unstructured) (*v1alpha1.ModuleTemplate, error) {
	var moduleTemplate v1alpha1.ModuleTemplate
	err := readModuleTemplate(module, &moduleTemplate)
	if err != nil {
		return &moduleTemplate, err
	}
	moduleTemplate.Name = module.Name
	moduleTemplate.Labels[v1alpha1.ModuleName] = module.Name
	moduleTemplate.Labels[v1alpha1.ControllerName] = module.ControllerName
	moduleTemplate.Spec.Channel = module.Channel
	if data.GetKind() != "" {
		moduleTemplate.Spec.Data = data
	}
	return &moduleTemplate, nil
}

func readModuleTemplate(module v1alpha1.Module, moduleTemplate *v1alpha1.ModuleTemplate) error {
	var template string
	switch module.ControllerName {
	case "manifest":
		template = "operator_v1alpha1_moduletemplate_skr-module.yaml"
	default:
		template = "operator_v1alpha1_moduletemplate_kcp-module.yaml"
	}
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		panic("Can't capture current filename!")
	}
	modulePath := filepath.Join(filepath.Dir(filename), "../../config/samples/component-integration-installed", template)

	moduleFile, err := os.ReadFile(modulePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(moduleFile, &moduleTemplate)
	return err
}
