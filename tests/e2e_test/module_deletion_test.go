package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Non Blocking Kyma Module Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			By("Then the Module Operator is deployed on the SKR cluster")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system", "apps", "v1",
					"Deployment", runtimeClient).
				Should(Succeed())
			By("And the KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the Kyma Module is disabled with an existing finalizer", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
				Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
				Should(Succeed())
			By("Then the KCP Kyma CR is in \"Processing\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateProcessing).
				Should(Succeed())
			By("And the Manifest CR is in \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
			By("And the Module CR on SKR Cluster is not removed")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io",
					"v1alpha1", "Sample", runtimeClient).
				Should(Succeed())
			By("And the Module CR on SKR Cluster is in \"Deleting\" State")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Deleting").
				Should(Succeed())
			Eventually(SampleCRDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient).
				Should(Succeed())

			By("And the Module Operator Deployment is not removed on the SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system",
					"apps", "v1", "Deployment", runtimeClient).
				Should(Succeed())
		})

		It("Given the Kyma Module is under deletion", func() {
			By("When the Kyma Module is re-enabled in a different Module Distribution Channel")
			module.Channel = "fast"
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			By("Then the Kyma Module is updated on the SKR Cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments("template-operator-v2-controller-manager", "template-operator-system", runtimeClient).
				Should(Succeed())
			By("And the old Module Operator Deployment is removed")
			Consistently(DeploymentIsReady).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system", runtimeClient).
				Should(Not(Succeed()))
			By("And the Module CR is in \"Deleting\" State")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Deleting").
				Should(Succeed())
			By("And the Manifest CR is still in \"Deleting\" State")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), "template-operator", controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("When the blocking finalizers from the Module CR get removed", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					[]string{}, runtimeClient).
				Should(Succeed())
			By("Then a new Module CR is created in SKR Cluster and in \"Ready\" State")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
					runtimeClient).
				Should(Succeed())
			Eventually(SampleCRNoDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient).
				Should(Succeed())
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Ready").
				Should(Succeed())
			By("And Module Operator Deployment is running")
			Consistently(DeploymentIsReady).
				WithContext(ctx).
				WithArguments("template-operator-v2-controller-manager", "template-operator-system", runtimeClient).
				Should(Succeed())
			By("And a new Manifest CR is created")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient, shared.StateReady).
				Should(Succeed())
			Eventually(ManifestNoDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient).
				Should(Succeed())
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})
		It("When the Kyma Module gets disabled",
			func() {
				Eventually(DisableModule).
					WithContext(ctx).
					WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
					Should(Succeed())
				By("Then the Module CR is removed")
				Eventually(CheckIfNotExists).
					WithContext(ctx).
					WithArguments("sample-yaml", "kyma-system",
						"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
					Should(Succeed())
				By("And the Module Operator Deployment is deleted")
				Eventually(CheckIfNotExists).
					WithContext(ctx).
					WithArguments("template-operator-v2-controller-manager",
						"template-operator-system", "apps", "v1", "Deployment", runtimeClient).
					Should(Succeed())
				By("And the Manifest CR is removed")
				Eventually(NoManifestExist).
					WithContext(ctx).
					WithArguments(controlPlaneClient).
					Should(Succeed())

				By("And the KCP Kyma CR is in \"Ready\" State")
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())

				By("And the SKR Kyma CR is in \"Ready\" State")
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(defaultRemoteKymaName, remoteNamespace, runtimeClient, shared.StateReady).
					Should(Succeed())
			})
	})
})
