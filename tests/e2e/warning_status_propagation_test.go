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

var _ = Describe("Warning Status Propagation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
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

			By("And Module in KCP Kyma CR is in \"Warning\" State")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(),
					module.Name, shared.StateWarning).
				Should(Succeed())

			By("And KCP kyma CR is in \"Warning\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateWarning).
				Should(Succeed())
		})

		It("When Kyma Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then there is no Module in KCP Kyma CR spec", func() {
			Eventually(NotContainsModuleInSpec, Timeout, Interval).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Warning\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateWarning).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateWarning).
				Should(Succeed())

			By("And Module Manifest CR is in a \"Warning\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateWarning).
				Should(Succeed())

			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateWarning).
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
