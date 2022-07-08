package util

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	"github.com/imdario/mergo"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/img"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrMoreThanOneConfigMapCandidate = errors.New("more than one config map candidate found")
	ErrNoConfigMapCandidate          = errors.New("no config map candidate found")
	ErrOverrideApply                 = errors.New("could not apply override")
)

type (
	ParsedModules map[string]*ParsedModule
	ParsedModule  struct {
		Name             string
		Template         *operatorv1alpha1.ModuleTemplate
		TemplateOutdated bool
		*unstructured.Unstructured
		Settings unstructured.Unstructured
	}
)

func (m *ParsedModule) Channel() operatorv1alpha1.Channel {
	return m.Template.Spec.Channel
}

func (m *ParsedModule) ApplyLabelsToUnstructured(
	kyma *operatorv1alpha1.Kyma,
	moduleName string,
) {
	lbls := m.Unstructured.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[labels.KymaName] = kyma.Name
	lbls[labels.Profile] = string(kyma.Spec.Profile)

	lbls[labels.ModuleName] = moduleName

	templateLabels := m.Template.GetLabels()
	if templateLabels != nil {
		lbls[labels.ControllerName] = m.Template.GetLabels()[labels.ControllerName]
	}
	lbls[labels.Channel] = string(m.Template.Spec.Channel)

	m.Unstructured.SetLabels(lbls)
}

func CopySettingsToUnstructuredFromResource(resource *unstructured.Unstructured,
	settings unstructured.Unstructured,
) error {
	overrideSpec := settings.Object["spec"]

	if overrideSpec != nil {
		if err := mergo.Merge(resource.Object["spec"], overrideSpec); err != nil {
			return err
		}
	}
	return nil
}

func ParseTemplates(kyma *operatorv1alpha1.Kyma, templates release.TemplatesInChannels,
	verificationFactory img.SignatureVerification,
) (ParsedModules, error) {
	// First, we fetch the module spec from the template and use it to resolve it into an arbitrary object
	// (since we do not know which module we are dealing with)
	modules := make(ParsedModules)

	var component *unstructured.Unstructured

	for _, module := range kyma.Spec.Modules {
		template := templates[module.Name]
		if template == nil {
			return nil, fmt.Errorf("could not find module %s for resource %s",
				module.Name, client.ObjectKeyFromObject(kyma))
		}

		var err error
		if component, err = GetUnstructuredComponentFromTemplate(template, module.Name,
			kyma, verificationFactory); err != nil {
			return nil, err
		}
		modules[module.Name] = &ParsedModule{
			Template:         template.Template,
			TemplateOutdated: template.Outdated,
			Unstructured:     component,
			Settings:         module.Settings,
		}
	}

	return modules, nil
}

func GetUnstructuredComponentFromTemplate(
	template *release.TemplateInChannel,
	componentName string,
	kyma *operatorv1alpha1.Kyma,
	signatureVerification img.SignatureVerification,
) (*unstructured.Unstructured, error) {
	component := &template.Template.Spec.Data
	component.SetName(componentName + kyma.Name)
	component.SetNamespace(kyma.GetNamespace())

	if template.Template.Spec.Descriptor.String() == "" {
		return component, nil
	}

	var descriptor ocm.ComponentDescriptor
	if err := codec.Decode(template.Template.Spec.Descriptor.Raw, &descriptor); err != nil {
		return nil, fmt.Errorf("error while decoding the descriptor: %w", err)
	}

	layers, err := img.VerifyAndParse(&descriptor, signatureVerification)
	if err != nil {
		return nil, fmt.Errorf("error while parsing descriptor in module template: %w", err)
	}

	for name, layer := range layers {
		var mergeData any
		install := map[string]any{
			"name": string(name),
			"type": layer.LayerType,
		}

		if err := mergo.Merge(&install, layer.LayerRepresentation.ToGenericRepresentation()); err != nil {
			return nil, err
		}

		if name == "config" {
			r := layer.LayerRepresentation.(*img.OCIRef)
			mergeData = map[string]any{"config": map[string]any{
				"repo":   r.Repo,
				"module": r.Module,
				"ref":    r.Digest,
			}}
		} else {
			mergeData = map[string]any{"installs": []map[string]any{install}}
		}

		if err := mergo.Merge(&component.Object, map[string]any{"spec": mergeData}); err != nil {
			return nil, err
		}
	}

	return component, nil
}

func ProcessModuleOverridesOnKyma(
	ctx context.Context, clnt client.Client, kyma *operatorv1alpha1.Kyma, modules ParsedModules,
) error {
	for _, moduleSpec := range kyma.Spec.Modules {
		if len(moduleSpec.Overrides) < 1 {
			continue
		}

		module, found := modules[moduleSpec.Name]
		if !found {
			continue
		}

		for _, override := range moduleSpec.Overrides {
			configMap, err := GetConfigMapFromLabelSelector(ctx, clnt, override.LabelSelector)
			if err != nil {
				return fmt.Errorf("error fetching config map from override selector: %w", err)
			}

			if err := ProcessOverrideConfigMap(module, override, configMap); err != nil {
				return fmt.Errorf("error while processing config map for override: %w", err)
			}

			if err := UpdateKymaControllerRefToConfigMap(ctx, clnt, kyma, configMap); err != nil {
				return fmt.Errorf("error setting config map controller reference: %w", err)
			}
		}
	}
	return nil
}

func ProcessOverrideConfigMap(
	module *ParsedModule, override operatorv1alpha1.Override, configMap *v1.ConfigMap,
) error {
	var overrideType string
	if overrideTypeFromLabel, found := configMap.
		GetLabels()[labels.OverrideType]; !found || overrideTypeFromLabel == "" {
		overrideType = operatorv1alpha1.OverrideTypeHelmValues
	} else {
		overrideType = overrideTypeFromLabel
	}
	if overrideType == operatorv1alpha1.OverrideTypeHelmValues {
		spec, specFound := module.Object["spec"].(map[string]any)
		if !specFound {
			return fmt.Errorf("error while applying override to .spec.installs[%s]: %w",
				override.Name, ErrOverrideApply)
		}
		installs, installsFound := spec["installs"].([]map[string]any)
		if !installsFound {
			return fmt.Errorf("error while applying override to .spec.installs[%s]: %w",
				override.Name, ErrOverrideApply)
		}
		for _, install := range installs {
			if install["name"] == override.Name {
				install["overrideRef"] = map[string]any{
					"name":      configMap.GetName(),
					"namespace": configMap.GetNamespace(),
				}
			}
		}
	}
	return nil
}

func GetConfigMapFromLabelSelector(
	ctx context.Context, clnt client.Client, labelSelector *metav1.LabelSelector,
) (*v1.ConfigMap, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("selector invalid: %w", err)
	}
	overrideConfigMaps := &v1.ConfigMapList{}
	if err := clnt.List(ctx, overrideConfigMaps,
		client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, fmt.Errorf("error while fetching config map: %w", err)
	}

	if len(overrideConfigMaps.Items) > 1 {
		return nil, fmt.Errorf("selector %s invalid: %w",
			selector.String(), ErrMoreThanOneConfigMapCandidate)
	} else if len(overrideConfigMaps.Items) == 0 {
		return nil, fmt.Errorf("selector %s invalid: %w",
			selector.String(), ErrNoConfigMapCandidate)
	}

	usedConfigMap := &overrideConfigMaps.Items[0]

	if l := usedConfigMap.GetLabels(); l == nil {
		usedConfigMap.SetLabels(make(map[string]string))
	}

	return usedConfigMap, nil
}

func UpdateKymaControllerRefToConfigMap(
	ctx context.Context, clnt client.Client, kyma *operatorv1alpha1.Kyma, configMap *v1.ConfigMap,
) error {
	// we now verify that we already own the config map
	previousOwnerRefs := len(configMap.GetOwnerReferences())
	if err := controllerutil.SetControllerReference(kyma, configMap, clnt.Scheme()); err != nil {
		return fmt.Errorf("override configuration could not be owned to watch for overrides: %w", err)
	}
	if previousOwnerRefs != len(configMap.GetOwnerReferences()) {
		if err := clnt.Update(ctx, configMap); err != nil {
			return fmt.Errorf("error updating newly set owner config map: %w", err)
		}
	}
	return nil
}
