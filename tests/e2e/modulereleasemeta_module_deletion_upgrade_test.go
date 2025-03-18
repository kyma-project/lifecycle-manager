package e2e_test

import (
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Kyma Module with ModuleReleaseMeta Upgrade Under Deletion", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module Operator is deployed on SKR cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace, "apps", "v1",
					"Deployment", skrClient).
				Should(Succeed())
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Kyma Module is disabled with existing finalizer", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io", "v1alpha1",
					string(templatev1alpha1.SampleKind),
					[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, skrClient).
				Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in \"Processing\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateProcessing).
				Should(Succeed())

			By("And Manifest CR is in \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())

			By("And Module CR on SKR Cluster is not removed")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io",
					"v1alpha1", string(templatev1alpha1.SampleKind), skrClient).
				Should(Equal(ErrDeletionTimestampFound))

			By("And Module Operator Deployment is not removed on SKR Cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", skrClient).
				Should(Succeed())
		})

		It("When ModuleReleaseMeta is updated with new version", func() {
			Eventually(UpdateChannelVersionInModuleReleaseMeta).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, NewerVersion).
				Should(Succeed())
		})

		It("Then Kyma Module is updated on SKR Cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And old Module Operator Deployment is removed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))
			Consistently(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And Module CR is in \"Deleting\" State")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, skrClient, shared.StateDeleting).
				Should(Succeed())

			By("And Manifest CR is still in \"Deleting\" State")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), TestModuleName, kcpClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("When blocking finalizers from Module CR get removed", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io", "v1alpha1",
					string(templatev1alpha1.SampleKind),
					[]string{}, skrClient).
				Should(Succeed())
		})

		It("Then Module CR has been deleted in SKR Cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io", "v1alpha1",
					string(templatev1alpha1.SampleKind),
					skrClient).
				Should(Equal(ErrNotFound))

			By("And Module Operator Deployment is deleted")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
