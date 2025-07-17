package e2e_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Manage Module Metrics", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And count of Kyma State Metric in \"Ready\" State is 1")
			Eventually(GetKymaStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), shared.StateReady).
				Should(Equal(1))

			By("And count of Kyma Module Metric in \"Ready\" State is 1")
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, shared.StateReady).
				Should(Equal(1))
		})

		It("Then Related Manifest Requeue Metrics Get Increased", func() {
			Eventually(IsManifestRequeueReasonCountIncreased).
				WithContext(ctx).
				WithArguments(string(metrics.ManifestAddFinalizer), string(queue.IntendedRequeue)).
				Should(BeTrue())
			Eventually(IsManifestRequeueReasonCountIncreased).
				WithContext(ctx).
				WithArguments(string(metrics.ManifestSyncResourcesEnqueueRequired), string(queue.IntendedRequeue)).
				Should(BeTrue())
		})

		It("When Kyma Module is disabled", func() {
			manifestInCluster, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
				module.Name)
			Expect(err).Should(Succeed())
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())

			By("Then Manifest CR is removed")
			Eventually(ManifestExistsByMetadata).
				WithContext(ctx).
				WithTimeout(2*time.Minute).
				WithArguments(kcpClient, manifestInCluster.Namespace, manifestInCluster.Name).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())

			By("And count of Kyma State Metric in \"Ready\" State is 1")
			Eventually(GetKymaStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), shared.StateReady).
				Should(Equal(1))

			By("And count of Kyma Module Metric in \"Ready\" State is 0")
			Eventually(GetModuleStateMetricCount).
				WithContext(ctx).
				WithArguments(kyma.GetName(), module.Name, shared.StateReady).
				Should(Equal(0))
		})

		It("Then Related Manifest Requeue Metrics Get Increased", func() {
			Eventually(IsManifestRequeueReasonCountIncreased).
				WithContext(ctx).
				WithArguments(string(metrics.ManifestPreDeleteEnqueueRequired), string(queue.IntendedRequeue)).
				Should(BeTrue())
			Eventually(IsManifestRequeueReasonCountIncreased).
				WithContext(ctx).
				WithArguments(string(metrics.ManifestReconcileFinished), string(queue.IntendedRequeue)).
				Should(BeTrue())
		})

		It("When KCP Kyma CR is deleted", func() {
			Eventually(DeleteKymaByForceRemovePurgeFinalizer).
				WithContext(ctx).
				WithArguments(kcpClient, kyma).
				Should(Succeed())
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient).
				Should(Succeed())
		})

		It("Then count of lifecycle_mgr_requeue_reason_total for kyma_deletion is 1", func() {
			Eventually(GetRequeueReasonCount).
				WithContext(ctx).
				WithArguments(string(metrics.KymaDeletion), string(queue.IntendedRequeue)).
				Should(Equal(1))

			By("And Kyma Metrics are removed")
			for _, state := range shared.AllKymaStates() {
				Eventually(AssertKymaStateMetricNotFound).
					WithContext(ctx).
					WithArguments(kyma.GetName(), state).
					Should(Equal(ErrMetricNotFound))
			}

			for _, state := range shared.AllModuleStates() {
				Eventually(GetModuleStateMetricCount).
					WithContext(ctx).
					WithArguments(kyma.GetName(), module.Name, state).
					Should(Equal(0))
			}
		})
	})
})
