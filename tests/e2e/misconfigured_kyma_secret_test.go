package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var _ = FDescribe("Misconfigured Kyma Secret", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	BeforeAll(func() {
		By("When a KCP Kyma CR is created on the KCP cluster with misconfigured Kyma Secret")
		Eventually(CreateInvalidKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
		Eventually(controlPlaneClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
	})
	CleanupKymaAfterAll(kyma)

	Context("Given Two Cluster Setup", func() {
		It("When Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module).
				Should(Succeed())
		})

		It("Then Module is not created", func() {
			Consistently(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Not(Succeed()))
			By("No Manifest CR exists")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(controlPlaneClient).
				Should(Succeed())
		})

		It("When Kyma Secret is corrected", func() {
			Eventually(DeleteKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

		It("Then Module is created", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
			By("Manifest CR is in \"Ready\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateReady).
				Should(Succeed())
		})
	})
})
