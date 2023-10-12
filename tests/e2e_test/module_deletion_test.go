package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Non Blocking Module Deletion", Ordered, func() {
	kyma := testutils.NewKymaWithSyncLabel("kyma-sample", "kcp-system", "fast",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)

	Context("Given a Kyma Cluster", func() {
		It("When its corresponding CR is created", func() {
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(controlPlaneClient.Create).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("Then the Kyma CR is in a `Ready` state on control-plane cluster")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
			By("And the Kyma CR is in `Ready` sate on runtime cluster")
			Eventually(CheckRemoteKymaCR).
				WithContext(ctx).
				WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
				Should(Succeed())
		})
		It("When a module named `template-operator` is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "fast", runtimeClient).
				Should(Succeed())
			By("Then the template-operator manager is deployed on runtime cluster")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system", "apps", "v1",
					"Deployment", runtimeClient).
				Should(Succeed())
			By("And the Kyma CR is in a `Ready` state")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})

		It("When the module `template-operator` is disabled with an existing finalizer", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
				Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
				Should(Succeed())
			By("Then Kyma CR is in state `Processing`")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateProcessing).
				Should(Succeed())
			By("And Manifest CR is in state `Deleting`")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), "template-operator", controlPlaneClient, v2.StateDeleting).
				Should(Succeed())
			By("And Sample CR is not removed and in state `Deleting`")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io",
					"v1alpha1", "Sample", runtimeClient).
				Should(Succeed())
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Deleting").
				Should(Succeed())
			Eventually(SampleCRDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient).
				Should(Succeed())

			By("And the template-operator manager Deployment is not removed on runtime cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system",
					"apps", "v1", "Deployment", runtimeClient).
				Should(Succeed())
			By("And it is possible to update the Sample CR in the runtime cluster")
			Eventually(UpdateSampleCRSpec).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "newValue", runtimeClient).
				Should(Succeed())
		})

		It("When the module under deletion, template-operator, is re-enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "fast", runtimeClient).
				Should(Succeed())
			By("Then the Manifest CR is still in Deleting state")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), "template-operator", controlPlaneClient, v2.StateDeleting).
				Should(Succeed())
			By("And the Sample CR is still in Deleting state")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Deleting").
				Should(Succeed())
		})

		It("When the Sample CR gets removed by manually deleting all blocking finalizers", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					[]string{}, runtimeClient).
				Should(Succeed())
			By("Then a New Sample CR is created")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					runtimeClient).
				Should(Succeed())
			Eventually(SampleCRNoDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient).
				Should(Succeed())
			By("And a New Manifest CR is created.")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), "template-operator", controlPlaneClient, v2.StateReady).
				Should(Succeed())
			Eventually(ManifestNoDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments(kyma.GetName(), "template-operator", controlPlaneClient).
				Should(Succeed())
			By("And Kyma CR is set to `Ready` state")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})
		It("When the module template-operator gets disabled",
			func() {
				Eventually(DisableModule).
					WithContext(ctx).
					WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
					Should(Succeed())
				By("Then Sample CR is removed")
				Eventually(CheckIfNotExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system",
						"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
					Should(Succeed())
				By("And template-operator deployment is deleted")
				Eventually(CheckIfNotExists).
					WithContext(ctx).
					WithArguments("template-operator-controller-manager",
						"template-operator-system", "apps", "v1", "Deployment", runtimeClient).
					Should(Succeed())
				By("And Kyma CR is set to `Ready` state")
				Eventually(testutils.IsKymaInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
					Should(Succeed())
			})

		It("When Kyma gets deleted", func() {
			Eventually(controlPlaneClient.Delete).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("Then Kyma CR should be removed")
			Eventually(CheckKCPKymaCRDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})
	})
})
