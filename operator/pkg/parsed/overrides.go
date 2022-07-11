package parsed

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const HashFormatBase = 10

var (
	ErrMoreThanOneConfigMapCandidate = errors.New("more than one config map candidate found")
	ErrNoConfigMapCandidate          = errors.New("no config map candidate found")
	ErrOverrideApply                 = errors.New("could not apply override")
)

func ProcessModuleOverridesOnKyma(
	ctx context.Context, clnt client.Client, kyma *v1alpha1.Kyma, modules Modules,
) error {
	if kyma.Status.ActiveOverrides == nil {
		kyma.Status.ActiveOverrides = make(map[string]*v1alpha1.ActiveOverride)
	}
	for _, moduleSpec := range kyma.Spec.Modules {
		if len(moduleSpec.Overrides) < 1 {
			continue
		}

		module, found := modules[moduleSpec.Name]
		if !found {
			continue
		}

		for _, override := range moduleSpec.Overrides {
			var overrideHash string
			var err error

			if overrideHash, err = ProcessOverride(module, override); err != nil {
				return fmt.Errorf("error while processing config map for override: %w", err)
			}

			active, overridePresent := kyma.Status.ActiveOverrides[moduleSpec.Name]
			if !overridePresent {
				active = &v1alpha1.ActiveOverride{}
			}
			if active.Hash != overrideHash {
				active.Applied = false
				active.Hash = overrideHash
			}
		}
	}
	return nil
}

func ProcessOverride(
	module *Module, override v1alpha1.Override,
) (string, error) {
	spec, specFound := module.Object["spec"].(map[string]any)
	if !specFound {
		return "", fmt.Errorf("error while applying override to .spec.installs[%s]: %w",
			override.Name, ErrOverrideApply)
	}

	installs, installsFound := spec["installs"].([]map[string]any)
	if !installsFound {
		return "", fmt.Errorf("error while applying override to .spec.installs[%s]: %w",
			override.Name, ErrOverrideApply)
	}

	hash, err := QuickHash(ApplyOverrideSelectors(override, installs))
	if err != nil {
		return "", fmt.Errorf("error in override hash calculation: %w", err)
	}

	return strconv.FormatUint(hash, HashFormatBase), nil
}

func QuickHash(s string) (uint64, error) {
	h := fnv.New64a()
	if _, err := h.Write([]byte(s)); err != nil {
		return 0, fmt.Errorf("failed to calculate quick hash: %w", err)
	}
	return h.Sum64(), nil
}

func ApplyOverrideSelectors(override v1alpha1.Override, installs []map[string]any) string {
	var overrideID string
	for _, install := range installs {
		if install["name"] == override.Name {
			install["overrideSelector"] = override.LabelSelector.DeepCopy()
			overrideID += override.LabelSelector.String()
		}
	}
	return overrideID
}
