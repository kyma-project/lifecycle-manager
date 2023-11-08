package e2e_test

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Module Without Default CR", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel, v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given Template Operator without Default CR", func() {
		It("When enable Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
		})

		It("Then no resources in manifest CR", func() {
			Eventually(func(g Gomega, ctx context.Context) error {
				_, err := GetManifestResource(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
				return err
			}).WithContext(ctx).Should(Equal(ErrManifestResourceIsNil))
		})

		It("Then no module CR exist", func() {
			moduleCR := NewTestModuleCR(remoteNamespace)
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Equal(ErrNotFound))
			Consistently(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Equal(ErrNotFound))
		})

		It("Then module state of KCP Kyma in Ready", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateReady).
				Should(Succeed())
		})

		It("Then state of KCP kyma in Ready", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
