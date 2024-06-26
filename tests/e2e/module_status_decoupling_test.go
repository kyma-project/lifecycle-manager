package e2e_test

import (
	"context"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Module Status Decoupling", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleWrongConfig := NewTestModuleWithFixName("template-operator-misconfigured", "regular")
	moduleCR := NewTestModuleCR(RemoteNamespace)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())

			By("And resource is defined in Manifest CR")
			Eventually(func(g Gomega, ctx context.Context) {
				resource, err := GetManifestResource(ctx, controlPlaneClient,
					kyma.GetName(), kyma.GetNamespace(), module.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(resource.GetName()).To(Equal(moduleCR.GetName()))
				Expect(resource.GetNamespace()).To(Equal(moduleCR.GetNamespace()))
				Expect(resource.GroupVersionKind().Version).To(Equal(moduleCR.GroupVersionKind().Version))
				Expect(resource.GroupVersionKind().Group).To(Equal(moduleCR.GroupVersionKind().Group))
				Expect(resource.GroupVersionKind().Kind).To(Equal(moduleCR.GroupVersionKind().Kind))
			}).WithContext(ctx).Should(Succeed())

			By("And Module in KCP Kyma CR is in \"Ready\" State")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(),
					module.Name, shared.StateReady).
				Should(Succeed())

			By("And KCP kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Kyma Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in a \"Processing\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateProcessing).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateProcessing).
				Should(Succeed())

			By("And Module Manifest CR is in a \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("When blocking finalizers from Module CR get removed", func() {
			var finalizers []string
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io", "v1alpha1",
					string(templatev1alpha1.SampleKind), finalizers, runtimeClient).
				Should(Succeed())
		})

		It("Then Module CR, Module Operator Deployment and Manifest CR are removed", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					"operator.kyma-project.io", "v1alpha1", string(templatev1alpha1.SampleKind), runtimeClient).
				Should(Equal(ErrNotFound))

			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentName, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(controlPlaneClient).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})
	})

	Context("Given SKR Cluster", func() {
		It("When Kyma Module with wrong configured image is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, moduleWrongConfig).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())

			By("And resource is defined in Manifest CR")
			Eventually(func(g Gomega, ctx context.Context) {
				resource, err := GetManifestResource(ctx, controlPlaneClient,
					kyma.GetName(), kyma.GetNamespace(), module.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(resource.GetName()).To(Equal(moduleCR.GetName()))
				Expect(resource.GetNamespace()).To(Equal(moduleCR.GetNamespace()))
				Expect(resource.GroupVersionKind().Version).To(Equal(moduleCR.GroupVersionKind().Version))
				Expect(resource.GroupVersionKind().Group).To(Equal(moduleCR.GroupVersionKind().Group))
				Expect(resource.GroupVersionKind().Kind).To(Equal(moduleCR.GroupVersionKind().Kind))
			}).WithContext(ctx).Should(Succeed())

			By("And Module in KCP Kyma CR is in \"Error\" State")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(),
					module.Name, shared.StateError).
				Should(Succeed())

			By("And KCP kyma CR is in \"Error\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateError).
				Should(Succeed())
		})

		It("When Kyma Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in a \"Processing\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateProcessing).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateProcessing).
				Should(Succeed())

			By("And Module Manifest CR is in a \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("When blocking finalizers from Module CR get removed", func() {
			var finalizers []string
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io", "v1alpha1",
					string(templatev1alpha1.SampleKind), finalizers, runtimeClient).
				Should(Succeed())
		})

		It("Then Module CR, Module Operator Deployment and Manifest CR are removed", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					"operator.kyma-project.io", "v1alpha1", string(templatev1alpha1.SampleKind), runtimeClient).
				Should(Equal(ErrNotFound))

			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(runtimeClient, ModuleDeploymentName, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(controlPlaneClient).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
