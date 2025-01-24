package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

const fastChannel = "fast"

var _ = Describe("Maintenance Window With ModuleReleaseMeta and Module Downtime", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	kyma.Labels[shared.RegionLabel] = "europe"
	kyma.Spec.SkipMaintenanceWindows = false
	/*
		Maintenance Windows are defined as such:
			region asia: current time - current time + 2 hours
			region europe: tomorrow
	*/

	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster with Kyma CR NOT skipping maintenance windows and NO active maintenance window", func() {
		It("When Kyma Module requiring downtime is enabled without existing installation", func() {
			enableModule(&module, moduleCR, kyma, fastChannel, ModuleDeploymentNameInNewerVersion)
		})

		It("When Kyma Module requiring downtime is disabled again", func() {
			disableModule(&module, kyma)
		})

		It("When Kyma Module NOT requiring downtime is enabled", func() {
			enableModule(&module, moduleCR, kyma, shared.DefaultRemoteKymaName, ModuleDeploymentNameInOlderVersion)
		})

		It("When Kyma channel is changed leading to an update requiring downtime", func() {
			updateKymaChannel(moduleCR, kyma, fastChannel)
		})

		It("When Maintenance Window becomes active", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, shared.RegionLabel, "asia").
				Should(Succeed())

			By("Then Module CR exists")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And old Module Operator Deployment no longer exists")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
				Should(Equal(ErrNotFound))

			By("And new Module Operator Deployment is deployed")
			Eventually(DeploymentIsReady).
				WithContext(ctx).
				WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And Kyma Module Version in Kyma Status is updated")
			newModuleTemplateVersion, err := ReadModuleVersionFromModuleTemplate(ctx, kcpClient, module,
				kyma)
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

		It("When Kyma Module requiring downtime is disabled again", func() {
			disableModule(&module, kyma)
		})

		It("When Kyma is deleted", func() {
			Eventually(DeleteKyma, apimetav1.DeletePropagationBackground).
				WithContext(ctx).
				WithArguments(kcpClient, kyma).
				Should(Succeed())

			By("Then Kyma is removed from SKR")
			Eventually(KymaExists).
				WithContext(ctx).
				WithArguments(skrClient, shared.DefaultRemoteKymaName, shared.DefaultRemoteNamespace).
				Should(Succeed())
		})

		It("When Kyma is reinstalled with skipMaintenanceWindows=true and maintenance window NOT active", func() {
			kyma.Labels[shared.RegionLabel] = "europe"
			kyma.Spec.SkipMaintenanceWindows = true
			Eventually(kcpClient.Create).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())

			By("And SKR Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When Kyma Module NOT requiring downtime is enabled", func() {
			enableModule(&module, moduleCR, kyma, shared.DefaultRemoteKymaName, ModuleDeploymentNameInOlderVersion)
		})

		It("When Kyma channel is changed leading to an update requiring downtime", func() {
			updateKymaChannel(moduleCR, kyma, fastChannel)
		})
	})
})

func enableModule(module *v1beta2.Module,
	moduleCR *unstructured.Unstructured,
	kyma *v1beta2.Kyma,
	moduleChannel,
	moduleDeplyomentName string) {
	module.Channel = moduleChannel
	Eventually(EnableModule).
		WithContext(ctx).
		WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
		Should(Succeed())

	By("Then Module CR exists")
	Eventually(ModuleCRExists).
		WithContext(ctx).
		WithArguments(skrClient, moduleCR).
		Should(Succeed())

	By("And Module Operator Deployment is deployed")
	Eventually(DeploymentIsReady).
		WithContext(ctx).
		WithArguments(skrClient, moduleDeplyomentName, TestModuleResourceNamespace).
		Should(Succeed())

	By("And KCP Kyma CR is in \"Ready\" State")
	Eventually(KymaIsInState).
		WithContext(ctx).
		WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
		Should(Succeed())

	By("And SKR Kyma CR is in \"Ready\" State")
	Eventually(KymaIsInState).
		WithContext(ctx).
		WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
		Should(Succeed())
}

func disableModule(module *v1beta2.Module, kyma *v1beta2.Kyma) {
	Eventually(DisableModule).
		WithContext(ctx).
		WithArguments(skrClient, shared.DefaultRemoteKymaName, shared.DefaultRemoteNamespace, module.Name).
		Should(Succeed())

	By("Then Module Operator Deployment is removed")
	Eventually(GetDeployment).
		WithContext(ctx).
		WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
		Should(Equal(ErrNotFound))

	By("And KCP Kyma CR is in \"Ready\" State")
	Eventually(KymaIsInState).
		WithContext(ctx).
		WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
		Should(Succeed())
}

func updateKymaChannel(moduleCR *unstructured.Unstructured,
	kyma *v1beta2.Kyma,
	channel string) {
	Eventually(UpdateKymaModuleChannel).
		WithContext(ctx).
		WithArguments(skrClient, shared.DefaultRemoteKymaName, shared.DefaultRemoteNamespace, channel).
		Should(Succeed())

	By("Then Module CR exists")
	Eventually(ModuleCRExists).
		WithContext(ctx).
		WithArguments(skrClient, moduleCR).
		Should(Succeed())

	By("And old Module Operator Deployment still exists")
	Consistently(DeploymentIsReady).
		WithContext(ctx).
		WithArguments(skrClient, ModuleDeploymentNameInOlderVersion, TestModuleResourceNamespace).
		Should(Succeed())

	By("And new Module Operator Deployment is not deployed")
	Consistently(DeploymentIsReady).
		WithContext(ctx).
		WithArguments(skrClient, ModuleDeploymentNameInNewerVersion, TestModuleResourceNamespace).
		Should(Equal(ErrNotFound))

	By("And KCP Kyma CR is in \"Ready\" State")
	Eventually(KymaIsInState).
		WithContext(ctx).
		WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
		Should(Succeed())
}
