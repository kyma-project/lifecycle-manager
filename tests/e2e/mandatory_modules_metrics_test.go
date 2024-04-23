package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Mandatory Module Metrics", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		const deployName = "template-operator-v1-controller-manager"
		It("Then mandatory module is installed on the SKR cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, deployName,
					TestModuleResourceNamespace).
				Should(Succeed())
			By("And the SKR Module Default CR is in a \"Ready\" State", func() {
				Eventually(CheckSampleCRIsInState).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system", runtimeClient, shared.StateReady).
					Should(Succeed())
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
			})

			By("And count of Mandatory Module State Metric in \"Ready\" State is 1", func() {
				Eventually(GetMandatoryModuleStateMetric).
					WithContext(ctx).
					WithArguments(kyma.GetName(), "template-operator", string(shared.StateReady)).
					Should(Equal(1))
			})

			By("And count of Mandatory ModuleTemplates Metric is 1", func() {
				Eventually(GetMandatoryModuleTemplateCountMetric).
					WithContext(ctx).
					Should(Equal(1))
			})
		})

		It("When the mandatory ModuleTemplate is removed", func() {
			Eventually(DeleteCR).
				WithContext(ctx).
				WithArguments(controlPlaneClient,
					&v1beta2.ModuleTemplate{
						ObjectMeta: apimetav1.ObjectMeta{
							Name:      "template-operator-mandatory",
							Namespace: "kcp-system",
						},
					}).
				Should(Succeed())
		})
		It("Then mandatory SKR module is removed", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, deployName,
					TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And the mandatory SKR Module Default CR is removed", func() {
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system",
						"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
					Should(Equal(ErrNotFound))
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
			})

			By("And count of Mandatory Module State Metric in \"Ready\" State is 0", func() {
				Eventually(GetMandatoryModuleStateMetric).
					WithContext(ctx).
					WithArguments(kyma.GetName(), "template-operator", string(shared.StateReady)).
					Should(Equal(0))
			})

			By("And count of Mandatory ModuleTemplates Metric is 0", func() {
				Eventually(GetMandatoryModuleTemplateCountMetric).
					WithContext(ctx).
					Should(Equal(0))
			})
		})
	})
})
