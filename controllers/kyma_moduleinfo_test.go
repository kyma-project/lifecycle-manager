package controllers_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
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
			if moduleStatus.State != v1beta1.StateReady {
				return fmt.Errorf("%w: %s", ErrModuleStatusNotInReady, moduleStatus.Name)
			}
		}
		return nil
	}
}

func updateAllModuleState(kymaName string, state v1beta1.State) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			if err := updateModuleState(createdKyma, module, state); err != nil {
				return err
			}
		}
		return nil
	}
}

func removeModule(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		Expect(err).ShouldNot(HaveOccurred())
		createdKyma.Spec.Modules = []v1beta1.Module{}
		return controlPlaneClient.Update(ctx, createdKyma)
	}
}

var _ = Describe("Test Kyma CR", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta1.Module{
			ControllerName: "manifest",
			Name:           NewUniqModuleName(),
			Channel:        v1beta1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test Modules",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When deploy module, expect number of Modules matches spec.modules",
			noCondition(), expectCorrectNumberOfmoduleStatus(kyma.Name)),
		Entry("When module state become ready, expect Modules state become ready",
			updateAllModuleState(kyma.Name, v1beta1.StateReady), expectmoduleStatusStateBecomeReady(kyma.Name)),
		Entry("When remove module in spec, expect number of Modules matches spec.modules",
			removeModule(kyma.Name), expectCorrectNumberOfmoduleStatus(kyma.Name)),
	)
})
