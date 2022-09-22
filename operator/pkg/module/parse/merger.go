package parse

import (
	"fmt"

	"github.com/imdario/mergo"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

func mergeResourceIntoSpec(resource, component *unstructured.Unstructured) error {
	if err := mergo.Merge(&component.Object,
		map[string]any{"spec": map[string]any{"resource": resource}},
		mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("error while merging the template spec.data into the spec: %w", err)
	}
	return nil
}

// mergeTargetIntoSpec uses a given v1alpha1.Target to set the remote attribute on an unstructured.
func mergeTargetIntoSpec(remote v1alpha1.Target, component *unstructured.Unstructured) error {
	if err := mergo.Merge(&component.Object,
		map[string]any{"remote": ConvertTargetToRemote(remote)},
		mergo.WithAppendSlice); err != nil {
		return fmt.Errorf("error while merging the remote attribute into the spec: %w", err)
	}
	return nil
}

func ConvertTargetToRemote(remote v1alpha1.Target) bool {
	var isRemoteInstall bool
	switch remote {
	case v1alpha1.TargetControlPlane:
		isRemoteInstall = false
	case v1alpha1.TargetRemote:
		isRemoteInstall = true
	}
	return isRemoteInstall
}
