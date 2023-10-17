package e2e_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Purge Controller", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	moduleName := "template-operator"
	moduleCR := NewTestModuleCR(remoteNamespace)
	moduleCRFinalizer := "cr-finalizer"

	InitEmptyKymaBeforeAll(kyma)

	Context("Given Template Operator", func() {
		It("When enable Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, kyma.Spec.Channel, runtimeClient).
				Should(Succeed())
		})

		It("Then module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})
	})

	Context("Given a module CR", func() {
		It("When a finalizer is added to Module CR", func() {
			Expect(AddFinalizerToModuleCR(ctx, runtimeClient, moduleCR, moduleCRFinalizer)).
				Should(Succeed())
		})

		It("And Kyma has deletion timestamp set", func() {
			Expect(DeleteKyma(ctx, controlPlaneClient, kyma)).
				Should(Succeed())

			Expect(KymaHasDeletionTimestamp(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())).
				Should(BeTrue())
		})

		It("Then finalizer is removed from module CR after purge timeout", func() {
			kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
			Expect(err).NotTo(HaveOccurred())

			deletionTimeout := kyma.DeletionTimestamp.Add(10 * time.Second)

			Eventually(FinalizerIsRemovedAfterTimeout).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR, moduleCRFinalizer, &deletionTimeout).
				Should(Succeed())
		})

		It("And module CR is deleted", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Equal(ErrNotFound))
		})

		It("And KCP and SKR Kymas are deleted", func() {
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, runtimeClient).
				Should(Equal(ErrNotFound))

			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Equal(ErrNotFound))
		})
	})
})
