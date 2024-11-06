package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Module Upgrade By New Version", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	newTemplateFilePath := "../moduletemplates/moduletemplate_template_operator_v2_regular_new_version.yaml"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And Module Operator Deployment exists")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Module Template Version is Upgraded", func() {
			Expect(ApplyYAML(ctx,
				kcpClient,
				newTemplateFilePath)).
				Should(Succeed())

			By("And ModuleReleaseMeta is updated if it exists")
			Eventually(UpdateChannelVersionIfModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace, v1beta2.DefaultChannel, "2.4.2-e2e-test").
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And new Module Operator Deployment exists")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And old Module Operator Deployment does not exist")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And Kyma Module Version in Kyma Status is updated")
			newModuleTemplateVersion, err := ReadModuleVersionFromModuleTemplate(ctx, kcpClient, module,
				kyma.Spec.Channel, ControlPlaneNamespace)
			Expect(err).ToNot(HaveOccurred())

			Eventually(ModuleVersionInKymaStatusIsCorrect).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					newModuleTemplateVersion).
				Should(Succeed())

			By("And Manifest Version is updated")
			Eventually(ManifestVersionIsCorrect).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					newModuleTemplateVersion).
				Should(Succeed())
		})
	})
})
