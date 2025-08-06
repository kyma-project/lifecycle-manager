package api

import (
	"fmt"

	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func AddToScheme(scheme *machineryruntime.Scheme) error {
	err := v1beta2.AddToScheme(scheme)
	if err != nil {
		return fmt.Errorf("failed to add scheme on v1beta2 api: %w", err)
	}

	return nil
}
