package controllers_test

import (
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func noCondition() func() error {
	return func() error {
		return nil
	}
}

func expectedCorrectNumberOfModuleInfo(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		Expect(err).ShouldNot(HaveOccurred())
		if len(createdKyma.Spec.Modules) == len(createdKyma.Status.ModuleInfos) {
			return nil
		}
		return errors.New("Spec.Modules number not match with Status.ModuleInfos")
	}
}

func expectedModuleInfoStateBecomeReady(kymaName string) func() error {
	return func() error {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		Expect(err).ShouldNot(HaveOccurred())
		for _, moduleInfo := range createdKyma.Status.ModuleInfos {
			if moduleInfo.State != v1alpha1.StateReady {
				return fmt.Errorf("moduleInfo: %s not in ready state", moduleInfo.Name)
			}
		}
		return nil
	}
}

func updateAllModuleState(kymaName string, state v1alpha1.State) func() error {
	return func() error {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		Expect(err).ShouldNot(HaveOccurred())
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

	moduleTemplates := make([]*v1alpha1.ModuleTemplate, 0)

	BeforeAll(func() {
		deployModuleTemplate(kyma, moduleTemplates)
	})
	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})
	DescribeTable("Test ModuleInfo",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, timeout, interval).Should(Succeed())
			Eventually(expectedBehavior, timeout, interval).Should(Succeed())
		},
		Entry("When deploy module, expect number of ModuleInfo matches spec.modules", noCondition(), expectedCorrectNumberOfModuleInfo(kyma.Name)),
		Entry("When module state become ready, expect ModuleInfo state become ready", updateAllModuleState(kyma.Name, v1alpha1.StateReady), expectedModuleInfoStateBecomeReady(kyma.Name)),
		Entry("When remove module in spec, expect number of ModuleInfo matches spec.modules", removeModule(kyma.Name), expectedCorrectNumberOfModuleInfo(kyma.Name)),
	)
})

func deployModuleTemplate(kyma *v1alpha1.Kyma, moduleTemplates []*v1alpha1.ModuleTemplate) {
	Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	for _, module := range kyma.Spec.Modules {
		template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
		moduleTemplates = append(moduleTemplates, template)
	}
}
