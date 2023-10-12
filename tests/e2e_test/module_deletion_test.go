package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Non Blocking Kyma Module Deletion", Ordered, func() {
	kyma := testutils.NewKymaWithSyncLabel("kyma-sample", "kcp-system", "fast",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)

	Context("Given an SKR cluster", func() {
		It("When its KCP Kyma CR is created in the KCP cluster", func() {
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(controlPlaneClient.Create).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("Then the Kyma CR is in `Ready` state in the KCP cluster ")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
			By("And the Kyma CR is in `Ready` in the SKR cluster")
			Eventually(CheckRemoteKymaCR).
				WithContext(ctx).
				WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
				Should(Succeed())
		})
		It("When a Kyma Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "fast", runtimeClient).
				Should(Succeed())
			By("Then the Module Operator is deployed on the SKR cluster")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system", "apps", "v1",
					"Deployment", runtimeClient).
				Should(Succeed())
			By("And the KCP Kyma CR is in a Ready State")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})

		It("When the Module is disabled with an existing finalizer", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
				Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
				Should(Succeed())
			By(" Then KCP Kyma CR is in a Processing State")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateProcessing).
				Should(Succeed())
			By("And the Module Manifest CR is in a Deleting State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), "template-operator", controlPlaneClient, v2.StateDeleting).
				Should(Succeed())
			By("And SKR Module Default CR is not removed and in state `Deleting`")
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

			By("And the Module Manager Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system",
					"apps", "v1", "Deployment", runtimeClient).
				Should(Succeed())
			By("And Module Default CR is mutable.")
			Eventually(UpdateSampleCRSpec).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "newValue", runtimeClient).
				Should(Succeed())
		})

		It("Given the Kyma Module is under deletion", func() {
			By("When the Module is re-enabled")
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "fast", runtimeClient).
				Should(Succeed())
			By("Then the Module Manifest CR is still in Deleting State")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), "template-operator", controlPlaneClient, v2.StateDeleting).
				Should(Succeed())
			By("And the Module Default CR is plucked outin Deleting state")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Deleting").
				Should(Succeed())
		})

		It("When the blocking finalizers from the Default Module CR get removed", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					[]string{}, runtimeClient).
				Should(Succeed())
			By("Then a New Default Module CR is created")
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
			By("And KCP Kyma CR is in a Ready State")
			Eventually(testutils.IsKymaInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})
		It("When the Kyma Module gets disabled",
			func() {
				Eventually(DisableModule).
					WithContext(ctx).
					WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
					Should(Succeed())
				By("Then Module Default CR is removed")
				Eventually(CheckIfNotExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system",
						"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
					Should(Succeed())
				By("And the Module Deployment is deleted")
				Eventually(CheckIfNotExists).
					WithContext(ctx).
					WithArguments("template-operator-controller-manager",
						"template-operator-system", "apps", "v1", "Deployment", runtimeClient).
					Should(Succeed())
				By(" And the SKR Kyma CR is in a Ready State")
				Eventually(testutils.IsKymaInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
					Should(Succeed())
			})

		AfterAll(func() {
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
