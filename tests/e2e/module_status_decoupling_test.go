package e2e_test

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

type ResourceKind string

const (
	DeploymentKind  ResourceKind = "Deployment"
	StatefulSetKind ResourceKind = "StatefulSet"
)

func RunModuleStatusDecouplingTest(resourceKind ResourceKind) {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleWrongConfig := NewTestModuleWithFixName(MisconfiguredModuleName, v1beta2.DefaultChannel, "")
	moduleCR := NewTestModuleCR(RemoteNamespace)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		checkModuleStatus(module, moduleCR, kyma, shared.StateReady)

		It("And Module Resource is ready", func() {
			switch resourceKind {
			case DeploymentKind:
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
					Should(Succeed())
			case StatefulSetKind:
				Eventually(StatefulSetIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
					Should(Succeed())
			}
		})

		It("When Kyma Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in a \"Processing\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateProcessing).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateProcessing).
				Should(Succeed())

			By("And Module Manifest CR is in a \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())
			By("And Module CR is in \"Warning\" State")
			Eventually(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR, shared.StateWarning).
				Should(BeTrue())
			Consistently(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR, shared.StateWarning).
				Should(BeTrue())
		})

		It("When blocking finalizers from Module CR get removed", func() {
			var finalizers []string
			Eventually(SetFinalizer).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io", "v1alpha1",
					string(templatev1alpha1.SampleKind), finalizers, skrClient).
				Should(Succeed())
		})

		It("Then Module CR, Module Operator Resource and Manifest CR are removed", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					"operator.kyma-project.io", "v1alpha1", string(templatev1alpha1.SampleKind), skrClient).
				Should(Equal(ErrNotFound))

			switch resourceKind {
			case DeploymentKind:
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			case StatefulSetKind:
				Eventually(StatefulSetIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			}

			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})

	Context("Given SKR Cluster", func() {
		It("When Kyma Module with wrong configured image is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, moduleWrongConfig).
				Should(Succeed())
		})

		checkModuleStatus(moduleWrongConfig, moduleCR, kyma, shared.StateError)

		It("When Kyma Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, moduleWrongConfig.Name).
				Should(Succeed())
		})

		It("Then Module CR, Module Operator Deployment and Manifest CR are removed", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					"operator.kyma-project.io", "v1alpha1", string(templatev1alpha1.SampleKind), skrClient).
				Should(Equal(ErrNotFound))

			switch resourceKind {
			case DeploymentKind:
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			case StatefulSetKind:
				Eventually(StatefulSetIsReady).
					WithContext(ctx).
					WithArguments(skrClient, ModuleResourceName, TestModuleResourceNamespace).
					Should(Equal(ErrNotFound))
			}

			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})
}

func checkModuleStatus(module v1beta2.Module, moduleCR *unstructured.Unstructured, kyma *v1beta2.Kyma,
	expectedState shared.State,
) {
	It("Then Module CR exists", func() {
		Eventually(ModuleCRExists).
			WithContext(ctx).
			WithArguments(skrClient, moduleCR).
			Should(Succeed())

		By("And resource is defined in Manifest CR")
		Eventually(func(g Gomega, ctx context.Context) {
			resource, err := GetManifestResource(ctx, kcpClient,
				kyma.GetName(), kyma.GetNamespace(), module.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(resource.GetName()).To(Equal(moduleCR.GetName()))
			Expect(resource.GetNamespace()).To(Equal(moduleCR.GetNamespace()))
			Expect(resource.GroupVersionKind().Version).To(Equal(moduleCR.GroupVersionKind().Version))
			Expect(resource.GroupVersionKind().Group).To(Equal(moduleCR.GroupVersionKind().Group))
			Expect(resource.GroupVersionKind().Kind).To(Equal(moduleCR.GroupVersionKind().Kind))
		}).WithContext(ctx).Should(Succeed())

		By("And Module in KCP Kyma CR is in " + string(expectedState) + " State")
		Eventually(CheckModuleState).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(),
				module.Name, expectedState).
			Should(Succeed())

		By("And KCP kyma CR is in " + string(expectedState) + " State")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, expectedState).
			Should(Succeed())
	})
}
