package kyma_controller_test

import (
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func expectManifestSpecDataEquals(kymaName, value string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, metav1.NamespaceDefault)
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			if KCPModuleExistWithOverwrites(createdKyma, module) != value {
				return ErrSpecDataMismatch
			}
		}
		return nil
	}
}
