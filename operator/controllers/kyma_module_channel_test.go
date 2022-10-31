package controllers_test

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/controllers/testhelper"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Switching of a Channel leading to an Upgrade", Ordered, func() {
	kyma := testhelper.NewTestKyma("empty-module-kyma")

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "channel-switch",
		Channel:        v1alpha1.ChannelStable,
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})

	BeforeAll(SetupModuleTemplateSetsForKyma(kyma))
	AfterAll(CleanupModuleTemplateSetsForKyma(kyma))

	It(
		"should create kyma with standard modules in stable normally", func() {
			Expect(controlPlaneClient.Create(ctx, kyma)).ToNot(HaveOccurred())
			Eventually(GetKymaState(kyma.Name), 5*time.Second, interval).
				Should(BeEquivalentTo(string(v1alpha1.StateProcessing)))
			for _, module := range kyma.Spec.Modules {
				Eventually(
					UpdateModuleState(kyma.GetName(), module.Name, v1alpha1.StateReady), 20*time.Second, interval,
				).
					Should(Succeed())
			}
			Eventually(GetKymaState(kyma.Name), 5*time.Second, interval).
				Should(BeEquivalentTo(string(v1alpha1.StateReady)))
		},
	)

	DescribeTable(
		"Test Channel Status", func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, timeout, interval).Should(Succeed())
			Eventually(expectedBehavior, timeout, interval).Should(Succeed())
		},
		Entry(
			"When kyma is deployed in stable channel, expect ModuleStatus to be in stable channel",
			testhelper.NoCondition(),
			expectEveryModuleStatusToHaveChannel(kyma.Name, v1alpha1.ChannelStable),
		),
		Entry(
			"When all modules are updated to fast channel, expect ModuleStatus to update to fast channel",
			whenUpdatingEveryModuleChannel(kyma.Name, v1alpha1.ChannelFast),
			expectEveryModuleStatusToHaveChannel(kyma.Name, v1alpha1.ChannelFast),
		),
		Entry(
			"When all modules are reverted to stable channel, expect ModuleStatus to stay in fast channel",
			whenUpdatingEveryModuleChannel(kyma.Name, v1alpha1.ChannelStable),
			expectEveryModuleStatusToHaveChannel(kyma.Name, v1alpha1.ChannelFast),
		),
	)

	It(
		"should lead to kyma being ready in the end of the channel switch", func() {
			By("having updated the Kyma CR state to ready")
			Eventually(GetKymaState(kyma.Name), 20*time.Second, interval).
				Should(BeEquivalentTo(string(v1alpha1.StateReady)))
		},
	)
},
)

func SetupModuleTemplateSetsForKyma(kyma *v1alpha1.Kyma) func() {
	return func() {
		By("creating decremented ModuleTemplate set in stable")
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
			By("decrementing the module template from the factory in the patch of the semantic version")
			Expect(
				template.Spec.ModifyDescriptor(
					v1alpha1.ModifyDescriptorVersion(
						func(version *semver.Version) string {
							return fmt.Sprintf("v%v.%v.%v", version.Major(), version.Minor(), version.Patch()-1)
						},
					),
				),
			).ToNot(HaveOccurred())

			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
		}

		By("Creating standard ModuleTemplate set in fast")
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			template.Spec.Channel = v1alpha1.ChannelFast
			template.Name = fmt.Sprintf("%s-%s", template.Name, "fast")
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
		}
	}
}

func CleanupModuleTemplateSetsForKyma(kyma *v1alpha1.Kyma) func() {
	return func() {
		By("Cleaning up decremented ModuleTemplate set in stable")
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Delete(ctx, template)).To(Succeed())
		}
		By("Cleaning up standard ModuleTemplate set in fast")
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
			template.Name = fmt.Sprintf("%s-%s", template.Name, "fast")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Delete(ctx, template)).To(Succeed())
		}
	}
}

func expectEveryModuleStatusToHaveChannel(kymaName string, channel v1alpha1.Channel) func() error {
	return func() error {
		return TemplateInfosMatchChannel(kymaName, channel)
	}
}

func whenUpdatingEveryModuleChannel(kymaName string, channel v1alpha1.Channel) func() error {
	return func() error {
		return UpdateKymaModuleChannels(kymaName, channel)
	}
}
