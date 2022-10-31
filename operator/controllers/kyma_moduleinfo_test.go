package controllers_test

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ErrModuleNumberMismatch   = errors.New("Spec.Modules number not match with Status.ModuleStatus")
	ErrModuleStatusNotInReady = errors.New("moduleStatus not in ready state")
)

func expectCorrectNumberOfmoduleStatus(kymaName string) func() error {
	return func() error {
		createdKyma, err := testhelper.GetKyma(ctx, controlPlaneClient, kymaName)
		if err != nil {
			return err
		}
		if len(createdKyma.Spec.Modules) == len(createdKyma.Status.ModuleStatus) {
			return nil
		}
		return ErrModuleNumberMismatch
	}
}

func expectmoduleStatusStateBecomeReady(kymaName string) func() error {
	return func() error {
		createdKyma, err := testhelper.GetKyma(ctx, controlPlaneClient, kymaName)
		if err != nil {
			return err
		}
		for _, moduleStatus := range createdKyma.Status.ModuleStatus {
			if moduleStatus.State != v1alpha1.StateReady {
				return fmt.Errorf("%w: %s", ErrModuleStatusNotInReady, moduleStatus.Name)
			}
		}
		return nil
	}
}

func updateAllModuleState(kymaName string, state v1alpha1.State) func() error {
	return func() error {
		createdKyma, err := testhelper.GetKyma(ctx, controlPlaneClient, kymaName)
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			if err := updateModuleState(kymaName, module.Name, state); err != nil {
				return err
			}
		}
		return nil
	}
}

func removeModule(kymaName string) func() error {
	return func() error {
		createdKyma, err := testhelper.GetKyma(ctx, controlPlaneClient, kymaName)
		Expect(err).ShouldNot(HaveOccurred())
		createdKyma.Spec.Modules = v1alpha1.Modules{}
		return controlPlaneClient.Update(ctx, createdKyma)
	}
}

var _ = Describe("Test Kyma CR", Ordered, func() {
	kyma := testhelper.NewTestKyma("kyma")

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           testhelper.NewUniqModuleName(),
		Channel:        v1alpha1.ChannelStable,
	})

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test ModuleStatus",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, timeout, interval).Should(Succeed())
			Eventually(expectedBehavior, timeout, interval).Should(Succeed())
		},
		Entry("When deploy module, expect number of ModuleStatus matches spec.modules",
			testhelper.NoCondition(), expectCorrectNumberOfmoduleStatus(kyma.Name)),
		Entry("When module state become ready, expect ModuleStatus state become ready",
			updateAllModuleState(kyma.Name, v1alpha1.StateReady), expectmoduleStatusStateBecomeReady(kyma.Name)),
		Entry("When remove module in spec, expect number of ModuleStatus matches spec.modules",
			removeModule(kyma.Name), expectCorrectNumberOfmoduleStatus(kyma.Name)),
	)
})
