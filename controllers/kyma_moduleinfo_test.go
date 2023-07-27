package controllers_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrModuleNumberMismatch   = errors.New("Spec.Modules number not match with Status.Modules")
	ErrModuleStatusNotInReady = errors.New("moduleStatus not in ready state")
)

func noCondition() func() error {
	return func() error {
		return nil
	}
}

func expectCorrectNumberOfmoduleStatus(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		if len(createdKyma.Spec.Modules) == len(createdKyma.Status.Modules) {
			return nil
		}
		return ErrModuleNumberMismatch
	}
}

func expectmoduleStatusStateBecomeReady(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, moduleStatus := range createdKyma.Status.Modules {
			if moduleStatus.State != v1beta2.StateReady {
				return fmt.Errorf("%w: %s", ErrModuleStatusNotInReady, moduleStatus.Name)
			}
		}
		return nil
	}
}

func removeModule(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		Expect(err).ShouldNot(HaveOccurred())
		createdKyma.Spec.Modules = []v1beta2.Module{}
		return controlPlaneClient.Update(ctx, createdKyma)
	}
}

var _ = Describe("Kyma module control", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, NewTestModule("module", v1beta2.DefaultChannel))

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test Manifests",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When deploy module, expect number of Manifests matches spec.modules",
			noCondition(), expectCorrectNumberOfmoduleStatus(kyma.Name)),
		Entry("When module state become ready, expect Manifests state become ready",
			UpdateAllManifestState(kyma.Name, v1beta2.StateReady), expectmoduleStatusStateBecomeReady(kyma.Name)),
		Entry("When remove module in spec, expect number of Manifests matches spec.modules",
			removeModule(kyma.Name), expectCorrectNumberOfmoduleStatus(kyma.Name)),
	)
})
