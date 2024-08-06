package e2e_test

import (
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Unmanaging Kyma Module", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	module := NewTemplateOperator(v1beta2.DefaultChannel)

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
				WithArguments(ModuleResourceName, TestModuleResourceNamespace, "apps", "v1",
					"Deployment", skrClient).
				Should(Succeed())
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Kyma Module is unmanaged", func() {
			Eventually(SetModuleUnmanaged).
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
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind), skrClient).
				Should(Equal(ErrDeletionTimestampFound))

			By("And Module Operator Deployment is not removed on SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleResourceName, TestModuleResourceNamespace,
					"apps", "v1", "Deployment", skrClient).
				Should(Succeed())

			By("And Manifest CR is removed")
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

		It("When Kyma Module is set to managed again", func() {
			Eventually(SetModuleManaged).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Manifest is re-created", func() {
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient, shared.StateReady).
				Should(Succeed())
			Eventually(ManifestNoDeletionTimeStampSet).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient).
				Should(Succeed())
		})
	})
})
