package restrictedmodule

import (
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var ErrSelectorParse = errors.New("failed to parse kymaSelector on ModuleReleaseMeta")

// RestrictedModuleMatch extracts the kymaSelector from a ModuleReleaseMeta
// and checks if it matches labels on a specified Kyma instance.
// A nil or empty selector returns false.
func RestrictedModuleMatch(mrm *v1beta2.ModuleReleaseMeta, kyma *v1beta2.Kyma) (bool, error) {
	if mrm == nil || kyma == nil {
		return false, nil
	}

	kymaSelector := mrm.Spec.KymaSelector
	if kymaSelector == nil {
		return false, nil
	}
	if len(kymaSelector.MatchLabels) == 0 && len(kymaSelector.MatchExpressions) == 0 {
		return false, nil
	}

	selector, err := apimetav1.LabelSelectorAsSelector(kymaSelector)
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrSelectorParse, err)
	}

	return selector.Matches(k8slabels.Set(kyma.Labels)), nil
}
