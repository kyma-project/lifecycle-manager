package parsed

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	ErrMoreThanOneConfigMapCandidate = errors.New("more than one config map candidate found")
	ErrNoConfigMapCandidate          = errors.New("no config map candidate found")
	ErrOverrideApply                 = errors.New("could not apply override")
)

func ProcessModuleOverridesOnKyma(
	ctx context.Context, clnt client.Client, kyma *v1alpha1.Kyma, modules Modules,
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
	module *Module, override v1alpha1.Override, configMap *corev1.ConfigMap,
) error {
	var overrideType string
	if overrideTypeFromLabel, found := configMap.
		GetLabels()[v1alpha1.OverrideTypeLabel]; !found || overrideTypeFromLabel == "" {
		overrideType = v1alpha1.OverrideTypeHelmValues
	} else {
		overrideType = overrideTypeFromLabel
	}
	if overrideType == v1alpha1.OverrideTypeHelmValues {
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
) (*corev1.ConfigMap, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, fmt.Errorf("selector invalid: %w", err)
	}
	overrideConfigMaps := &corev1.ConfigMapList{}
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
	ctx context.Context, clnt client.Client, kyma *v1alpha1.Kyma, configMap *corev1.ConfigMap,
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
