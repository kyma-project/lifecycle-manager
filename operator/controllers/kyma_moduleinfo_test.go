package controllers_test

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ErrModuleNumberMismatch = errors.New("Spec.Modules number not match with Status.ModuleInfos")
	ErrModuleInfoNotInReady = errors.New("moduleInfo not in ready state")
)

func noCondition() func() error {
	return func() error {
		return nil
	}
}

func expectCorrectNumberOfModuleInfo(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return err
		}
		if len(createdKyma.Spec.Modules) == len(createdKyma.Status.ModuleInfos) {
			return nil
		}
		return ErrModuleNumberMismatch
	}
}

func expectModuleInfoStateBecomeReady(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return err
		}
		for _, moduleInfo := range createdKyma.Status.ModuleInfos {
			if moduleInfo.State != v1alpha1.StateReady {
				return fmt.Errorf("%w: %s", ErrModuleInfoNotInReady, moduleInfo.Name)
			}
		}
		return nil
	}
}

func updateAllModuleState(kymaName string, state v1alpha1.State) func() error {
	return func() error {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
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
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		Expect(err).ShouldNot(HaveOccurred())
		createdKyma.Spec.Modules = v1alpha1.Modules{}
		return controlPlaneClient.Update(ctx, createdKyma.SetObservedGeneration())
	}
}

var _ = Describe("Test Kyma CR", Ordered, func() {
	kyma := NewTestKyma("kyma")

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           NewUniqModuleName(),
		Channel:        v1alpha1.ChannelStable,
	})

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test ModuleInfo",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, timeout, interval).Should(Succeed())
			Eventually(expectedBehavior, timeout, interval).Should(Succeed())
		},
		Entry("When deploy module, expect number of ModuleInfo matches spec.modules",
			noCondition(), expectCorrectNumberOfModuleInfo(kyma.Name)),
		Entry("When module state become ready, expect ModuleInfo state become ready",
			updateAllModuleState(kyma.Name, v1alpha1.StateReady), expectModuleInfoStateBecomeReady(kyma.Name)),
		Entry("When remove module in spec, expect number of ModuleInfo matches spec.modules",
			removeModule(kyma.Name), expectCorrectNumberOfModuleInfo(kyma.Name)),
	)
})
