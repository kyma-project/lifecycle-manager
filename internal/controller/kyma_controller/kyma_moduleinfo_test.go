package kyma_controller_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var ErrModuleNumberMismatch = errors.New("Spec.Modules number not match with Status.Modules")

func noCondition() func() error {
	return func() error {
		return nil
	}
}

func expectCorrectNumberOfModuleStatus(kymaName string) func() error {
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
	module := NewTestModule("module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(
		kyma.Spec.Modules, module)

	RegisterDefaultLifecycleForKyma(kyma)

	DescribeTable("Test Manifests",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When deploy module, expect number of Manifests matches spec.modules",
			noCondition(), expectCorrectNumberOfModuleStatus(kyma.Name)),
		Entry("When module state become ready, expect Manifests state become ready",
			UpdateAllManifestState(kyma.GetName(), kyma.GetNamespace(), shared.StateReady),
			expectKymaStatusModules(ctx, kyma, module.Name, shared.StateReady)),
		Entry("When remove module in spec, expect number of Manifests matches spec.modules",
			removeModule(kyma.Name), expectCorrectNumberOfModuleStatus(kyma.Name)),
	)
})
