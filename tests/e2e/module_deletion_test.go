package e2e_test

import (
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Non Blocking Kyma Module Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	ManagedCRKind := "Managed"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module Operator is deployed on SKR cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleDeploymentName, TestModuleResourceNamespace, "apps", "v1",
					"Deployment", runtimeClient).
				Should(Succeed())
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Kyma Module is disabled with existing finalizer", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version,
					string(templatev1alpha1.SampleKind),
					[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
				Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in \"Processing\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateProcessing).
				Should(Succeed())

			By("And Manifest CR is in \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())

			By("And Module CR on SKR Cluster is not removed")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind), runtimeClient).
				Should(Equal(ErrDeletionTimestampFound))

			By("And Module CR on SKR Cluster is in \"Deleting\" State")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, runtimeClient, shared.StateDeleting).
				Should(Succeed())
			Eventually(SampleCRDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, runtimeClient).
				Should(Succeed())

			By("And Module Operator Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleDeploymentName, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", runtimeClient).
				Should(Succeed())
		})

		It("When Kyma Module is re-enabled in different Module Distribution Channel", func() {
			module.Channel = "fast"
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Kyma Module is updated on SKR Cluster", func() {
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And old Module Operator Deployment is removed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentName, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))
			Consistently(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentName, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And Module CR is in \"Deleting\" State")
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, runtimeClient, shared.StateDeleting).
				Should(Succeed())

			By("And Manifest CR is still in \"Deleting\" State")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), "template-operator", controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("When blocking finalizers from Module CR get removed", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version,
					string(templatev1alpha1.SampleKind),
					[]string{}, runtimeClient).
				Should(Succeed())
		})

		It("Then new Module CR is created in SKR Cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version,
					string(templatev1alpha1.SampleKind),
					runtimeClient).
				Should(Succeed())
			Eventually(SampleCRNoDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, runtimeClient).
				Should(Succeed())

			By("And new Module CR is in \"Ready\" State")
			Eventually(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, runtimeClient, shared.StateReady).
				Should(Succeed())
			Consistently(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, runtimeClient, shared.StateReady).
				Should(Succeed())

			By("And Module Operator Deployment is running")
			Consistently(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And new Manifest CR is created")
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

		It("When Kyma Module gets disabled with managed CR blocked for deletion", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(ModuleManagedCRName, TestModuleResourceNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version,
					ManagedCRKind,
					[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
				Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then operator related resources not deleted", func() {
			By("And Manifest CR is in \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())

			By("And Module Operator Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", runtimeClient).
				Should(Succeed())
			By("And Module Operator RBAC related resources not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleServiceAccountName, TestModuleResourceNamespace,
					"", "v1", "ServiceAccount", runtimeClient).
				Should(Succeed())
		})

		It("When managed CR get deleted", func() {
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(ModuleManagedCRName, TestModuleResourceNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version,
					ManagedCRKind,
					[]string{}, runtimeClient).
				Should(Succeed())
		})

		It("Then Module CR is removed", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					templatev1alpha1.GroupVersion.Group, templatev1alpha1.GroupVersion.Version,
					string(templatev1alpha1.SampleKind),
					runtimeClient).
				Should(Equal(ErrNotFound))

			By("And Module Operator Deployment is deleted")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And Manifest CR is removed")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(controlPlaneClient).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())

			By("And SKR Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, runtimeClient, shared.StateReady).
				Should(Succeed())
		})

		It("When ModuleTemplate is removed from KCP Cluster", func() {
			Eventually(DeleteModuleTemplate).
				WithContext(ctx).
				WithArguments(controlPlaneClient, module, kyma.Spec.Channel).
				Should(Succeed())
		})

		It("Then ModuleTemplate is no longer in SKR Cluster", func() {
			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(runtimeClient, module, kyma.Spec.Channel).
				Should(Equal(ErrNotFound))
		})
	})
})
