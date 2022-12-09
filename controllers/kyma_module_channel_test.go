package controllers_test

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	FastChannel   = "fast"
	RandomChannel = "random"
	LowerVersion  = "0.0.1"
	HigherVersion = "0.0.2"
)

var _ = Describe("A random channel should be deployed successful", func() {
	kyma := NewTestKyma("kyma")
	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "module-with-random-channel",
			Channel:        RandomChannel,
		})

	BeforeEach(func() {
		SetupModuleTemplateSetsForKyma(kyma, LowerVersion, RandomChannel)
	})
	It("expect moduleStatus in random channel", func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).ToNot(HaveOccurred())
		Eventually(expectEveryModuleStatusToHaveChannel(kyma.Name, RandomChannel), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Switching of a Channel with higher version leading to an Upgrade", Ordered, func() {
	kyma := NewTestKyma("empty-module-kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "channel-switch",
			Channel:        v1alpha1.DefaultChannel,
		})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})

	BeforeAll(func() {
		SetupModuleTemplateSetsForKyma(kyma, LowerVersion, v1alpha1.DefaultChannel)
		SetupModuleTemplateSetsForKyma(kyma, HigherVersion, FastChannel)
	})

	AfterAll(CleanupModuleTemplateSetsForKyma(kyma))

	It(
		"should create kyma with standard modules in default channel normally", func() {
			Expect(controlPlaneClient.Create(ctx, kyma)).ToNot(HaveOccurred())
			Eventually(GetKymaState(kyma.Name), 5*time.Second, Interval).
				Should(BeEquivalentTo(string(v1alpha1.StateProcessing)))
			for _, module := range kyma.Spec.Modules {
				Eventually(
					UpdateModuleState(kyma.GetName(), module.Name, v1alpha1.StateReady), 20*time.Second,
					Interval).Should(Succeed())
			}
			Eventually(GetKymaState(kyma.Name), 5*time.Second, Interval).
				Should(BeEquivalentTo(string(v1alpha1.StateReady)))
		},
	)

	DescribeTable(
		"Test Channel Status", func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry(
			"When kyma is deployed in default channel with lower version,"+
				" expect ModuleStatus to be in regular channel",
			noCondition(),
			expectEveryModuleStatusToHaveChannel(kyma.Name, v1alpha1.DefaultChannel),
		),
		Entry(
			"When all modules are updated to fast channel with higher version,"+
				" expect ModuleStatus to update to fast channel",
			whenUpdatingEveryModuleChannel(kyma.Name, FastChannel),
			expectEveryModuleStatusToHaveChannel(kyma.Name, FastChannel),
		),
		Entry(
			"When all modules are reverted to regular channel,"+
				" expect ModuleStatus to stay in fast channel",
			whenUpdatingEveryModuleChannel(kyma.Name, v1alpha1.DefaultChannel),
			expectEveryModuleStatusToHaveChannel(kyma.Name, FastChannel),
		),
	)

	It(
		"should lead to kyma being ready in the end of the channel switch", func() {
			By("having updated the Kyma CR state to ready")
			Eventually(GetKymaState(kyma.Name), 20*time.Second, Timeout).
				Should(BeEquivalentTo(string(v1alpha1.StateReady)))
		},
	)
},
)

func SetupModuleTemplateSetsForKyma(kyma *v1alpha1.Kyma, modifiedVersion, channel string) {
	for _, module := range kyma.Spec.Modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
		Expect(
			template.Spec.ModifyDescriptor(
				v1alpha1.ModifyDescriptorVersion(
					func(version *semver.Version) string {
						return modifiedVersion
					},
				),
			),
		).ToNot(HaveOccurred())
		Expect(err).ShouldNot(HaveOccurred())
		template.Spec.Channel = channel
		template.Name = fmt.Sprintf("%s-%s", template.Name, channel)
		Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
	}
}

func CleanupModuleTemplateSetsForKyma(kyma *v1alpha1.Kyma) func() {
	return func() {
		By("Cleaning up decremented ModuleTemplate set in regular")
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			template.Name = fmt.Sprintf("%s-%s", template.Name, v1alpha1.DefaultChannel)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Delete(ctx, template)).To(Succeed())
		}
		By("Cleaning up standard ModuleTemplate set in fast")
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			template.Name = fmt.Sprintf("%s-%s", template.Name, FastChannel)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Delete(ctx, template)).To(Succeed())
		}
	}
}

func expectEveryModuleStatusToHaveChannel(kymaName, channel string) func() error {
	return func() error {
		return TemplateInfosMatchChannel(kymaName, channel)
	}
}

func whenUpdatingEveryModuleChannel(kymaName, channel string) func() error {
	return func() error {
		return UpdateKymaModuleChannels(kymaName, channel)
	}
}
