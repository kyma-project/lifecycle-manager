package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma Metrics", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)
	InitEmptyKymaBeforeAll(kyma)

	Context("Given kyma deployed in KCP", func() {
		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})

		It("Then KCP Kyma should be in Ready state", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("Then the count of kyma metric in ready equals 1", func() {
			Eventually(GetKymaStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), string(shared.StateReady)).
				Should(Equal(1))
		})

		It("And the count of kyma module metric in ready equals 1", func() {
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, string(shared.StateReady)).
				Should(Equal(1))
		})

		It("When disabling Template Operator", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
				Should(Succeed())

			By("Then the Manifest CR is removed")
			Eventually(ManifestExists).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
				Should(Equal(ErrNotFound))

			By("And the Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("Then the count of kyma metric in ready equals 1", func() {
			Eventually(GetKymaStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), string(shared.StateReady)).
				Should(Equal(1))
		})

		It("And the count of kyma module metric in ready equals 0", func() {
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, string(shared.StateReady)).
				Should(Equal(0))
		})

		It("When Kyma in KCP cluster is deleted", func() {
			Eventually(DeleteKymaByForceRemovePurgeFinalizer).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma).
				Should(Succeed())
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

		It("Then the count of metric should be removed", func() {
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
