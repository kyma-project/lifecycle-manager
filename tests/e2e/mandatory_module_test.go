package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Mandatory Module Installation and Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given kyma deployed in KCP", func() {
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
					WithArguments("sample-yaml", "kyma-system", runtimeClient, "Ready").
					Should(Succeed())
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
			})
		})

		It("When the mandatory Manifest is labelled to skip reconciliation", func() {
			Eventually(SetSkipLabelToMandatoryManifests).
				WithContext(ctx).
				WithArguments(controlPlaneClient, true).
				Should(Succeed())

			By("And deleting the mandatory SKR Default CR", func() {
				Eventually(DeleteCRWithGVK).
					WithContext(ctx).
					WithArguments(runtimeClient, "sample-yaml", "kyma-system", "operator.kyma-project.io",
						"v1alpha1", "Sample").
					Should(Succeed())
			})
			By("Then mandatory SKR Module Default CR is not recreated", func() {
				Consistently(CheckIfExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io",
						"v1alpha1", "Sample", runtimeClient).
					Should(Equal(ErrNotFound))
			})

			By("When deleting the mandatory SKR Module Manager Deployment", func() {
				err := DeleteCRWithGVK(ctx, runtimeClient, deployName,
					TestModuleResourceNamespace, "apps", "v1", "Deployment")
				Expect(err).ToNot(HaveOccurred())
			})
			By("Then Module Manager Deployment is not recreated on the SKR cluster", func() {
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(runtimeClient, deployName,
						TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
			})
		})

		It("When the mandatory Manifest skip reconciliation label is removed", func() {
			Eventually(SetSkipLabelToMandatoryManifests).
				WithContext(ctx).
				WithArguments(controlPlaneClient, false).
				Should(Succeed())

			By("Then mandatory SKR Module Default CR is recreated", func() {
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system",
						"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
					Should(Succeed())
			})
			By("Then mandatory SKR Module Deployment is recreated", func() {
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(runtimeClient, deployName,
						TestModuleResourceNamespace).
					Should(Succeed())
			})

			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
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
			By("Then mandatory SKR module is removed", func() {
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(runtimeClient, deployName,
						TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			})
			By("And the mandatory SKR Module Default CR is removed", func() {
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system",
						"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
					Should(Equal(ErrNotFound))
			})
			By("And the KCP Kyma CR is in a \"Ready\" State", func() {
				Consistently(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
			})
		})
	})
})
