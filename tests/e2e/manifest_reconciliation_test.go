package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Manifest Skip Reconciliation Label", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	waitingForFinalizersOperationMsg := "waiting as other finalizers are present"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given kyma deployed in KCP", func() {
		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			By("Then the Module Operator is deployed on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleResourceName,
					TestModuleResourceNamespace).
				Should(Succeed())
			By("And the SKR Module Default CR is in a \"Ready\" State")
			Eventually(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
			By("And the KCP Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And the Manifest contains the operator.kyma-project.io/channel label set to regular")
			Eventually(ManifestContainsExpectedLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					"operator.kyma-project.io/channel", "regular").
				Should(Succeed())
		})

		It("When the Manifest is labelled to skip reconciliation", func() {
			Eventually(SetSkipLabelToManifest).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, true).
				Should(Succeed())

			By("When deleting the SKR Default CR")
			Eventually(DeleteCRWithGVK).
				WithContext(ctx).
				WithArguments(skrClient, TestModuleCRName, RemoteNamespace, "operator.kyma-project.io",
					"v1alpha1", string(templatev1alpha1.SampleKind)).
				Should(Succeed())
			By("Then SKR Module Default CR is not recreated")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, "operator.kyma-project.io",
					"v1alpha1", string(templatev1alpha1.SampleKind), skrClient).
				Should(Equal(ErrNotFound))

			By("When deleting the SKR Module Manager Deployment")
			err := DeleteCRWithGVK(ctx, skrClient, ModuleResourceName,
				TestModuleResourceNamespace, "apps", "v1", "Deployment")
			Expect(err).ToNot(HaveOccurred())
			By("Then Module Manager Deployment is not recreated on the SKR cluster")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleResourceName,
					TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))
		})

		It("When the Manifest skip reconciliation label removed", func() {
			Eventually(SetSkipLabelToManifest).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, false).
				Should(Succeed())

			By("Then Module Default CR is recreated")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					"operator.kyma-project.io", "v1alpha1", string(templatev1alpha1.SampleKind), skrClient).
				Should(Succeed())
			By("Then Module Deployment is recreated")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleResourceName,
					TestModuleResourceNamespace).
				Should(Succeed())

			By("And the KCP Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And the SKR Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, RemoteNamespace, skrClient, shared.StateReady).
				Should(Succeed())
		})

		It("When a blocking finalizer is added to the Manifest CR", func() {
			Eventually(AddFinalizerToManifest).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					"blocking-finalizer").
				Should(Succeed())

			By("And Manifest CR has deletion timestamp set")
			Eventually(DeleteManifest).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
				Should(Succeed())
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("Then Manifest Status LastUpdateTime does not get changed", func() {
			Eventually(ManifestStatusOperationContainsMessage).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					waitingForFinalizersOperationMsg).
				Should(Succeed())

			manifest, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
			Expect(err).To(Not(HaveOccurred()))

			Consistently(ManifestStatusLastUpdateTimeIsNotChanged).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					manifest.Status).
				Should(Succeed())
		})
	})
})
