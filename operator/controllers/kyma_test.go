package controllers_test

import (
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func noCondition() func() bool {
	return func() bool {
		return true
	}
}

func expectedCorrectNumberOfModuleInfo(kymaName string) func() bool {
	return func() bool {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return false
		}
		return len(createdKyma.Spec.Modules) == len(createdKyma.Status.ModuleInfos)
	}
}

func removeModule(kymaName string) func() bool {
	return func() bool {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return false
		}
		createdKyma.Spec.Modules = v1alpha1.Modules{}
		err = controlPlaneClient.Update(ctx, createdKyma.SetObservedGeneration())
		if err != nil {
			return false
		}
		return true
	}
}

var _ = Describe("kyma", Ordered, func() {
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
	DescribeTable("Testing ModuleInfo",
		func(condition func() bool, expectedBehavior func() bool) {
			Eventually(condition, timeout*2, interval).Should(BeTrue())
			Eventually(expectedBehavior, timeout*2, interval).Should(BeTrue())
		},
		Entry("When init state, expect number of ModuleInfo matches spec.modules", noCondition(), expectedCorrectNumberOfModuleInfo(kyma.Name)),
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
