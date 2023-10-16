package e2e_test

import (
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

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

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
			Expect(AddFinalizerToModuleCR(ctx, runtimeClient, moduleCR, "cr-finalizer")).
				Should(Succeed())
		})

		It("And Kyma has deletion timestamp set", func() {
			Expect(DeleteKyma(ctx, controlPlaneClient, kyma)).
				Should(Succeed())

			Expect(KymaHasDeletionTimestamp(ctx, runtimeClient, kyma.GetName(), kyma.GetNamespace())).
				Should(BeTrue())
		})
	})
})
