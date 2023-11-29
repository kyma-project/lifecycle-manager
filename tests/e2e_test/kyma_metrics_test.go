package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Kyma Metrics", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)
	InitEmptyKymaBeforeAll(kyma)

	Context("Given an SKR Cluster", func() {
		It("When a Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())

			By("Then KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())

			By("And the count of Kyma State Metric in \"Ready\" State is 1")
			Eventually(GetKymaStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), string(shared.StateReady)).
				Should(Equal(1))

			By("And the count of Kyma Module Metric in \"Ready\" State is 1")
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, string(shared.StateReady)).
				Should(Equal(1))
		})

		It("When Kyma Module is disabled", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
				Should(Succeed())

			By("Then the Manifest CR is removed")
			Eventually(ManifestExists).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
				Should(Equal(ErrNotFound))

			By("And the KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())

			By("And the count of Kyma State Metric in \"Ready\" State is 1")
			Eventually(GetKymaStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), string(shared.StateReady)).
				Should(Equal(1))

			By("And the count of Kyma Module Metric in \"Ready\" State is 0")
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, string(shared.StateReady)).
				Should(Equal(0))
		})

		It("When KCP Kyma CR is deleted", func() {
			Eventually(DeleteKymaByForceRemovePurgeFinalizer).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma).
				Should(Succeed())
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())

			By("Then the count of all Kyma Metrics is 0")
			for _, state := range shared.AllStates() {
				Eventually(GetKymaStateMetricCount).
					WithContext(ctx).
					WithArguments(kyma.GetName(), string(state)).
					Should(Equal(0))
				Eventually(GetModuleStateMetricCount).
					WithContext(ctx).
					WithArguments(kyma.GetName(), module.Name, string(state)).
					Should(Equal(0))
			}
		})
	})
})
