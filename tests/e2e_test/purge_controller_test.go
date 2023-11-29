package e2e_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Purge Controller", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system",
		v1beta2.DefaultChannel, v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)
	InitEmptyKymaBeforeAll(kyma)

	Context("Given an SKR Cluster", func() {
		It("When a Kyma Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())

			By("Then Module CR exists")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})

		It("When a finalizer is added to Module CR", func() {
			Expect(AddFinalizerToModuleCR(ctx, runtimeClient, moduleCR, moduleCRFinalizer)).
				Should(Succeed())

			By("And KCP Kyma CR has deletion timestamp set")
			Expect(DeleteKyma(ctx, controlPlaneClient, kyma)).
				Should(Succeed())

			Expect(KymaHasDeletionTimestamp(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())).
				Should(BeTrue())

			By("Then finalizer is removed from Module CR after purge timeout")
			time.Sleep(5 * time.Second)
			Eventually(FinalizerIsRemoved).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR, moduleCRFinalizer).
				Should(Succeed())

			By("And Module CR is deleted")
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is deleted")
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())

			By("And SKR Kyma CR is deleted")
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, runtimeClient).
				Should(Succeed())
		})
	})
})
