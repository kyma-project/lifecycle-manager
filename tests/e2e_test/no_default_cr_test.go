package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Module Without Default CR", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular", v1beta2.SyncStrategyLocalSecret)
	moduleName := "template-operator"

	It("Given empty Kyma CR on remote cluster", func() {
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
		Eventually(controlPlaneClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("Then state of KCP Kyma in Ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
		By("Then state of SKR Kyma in Ready")
		Eventually(CheckRemoteKymaCR).
			WithContext(ctx).
			WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
			Should(Succeed())
	})

	Context("Given Template Operator without Default CR", func() {
		It("When enable Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, "regular", runtimeClient).
				Should(Succeed())
		})

		It("Then no module CR exist", func() {
			moduleCR := builder.NewModuleCRBuilder().Build()
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(controlPlaneClient, moduleCR.GetName(), moduleCR.GetNamespace()).
				Should(Equal(ErrNotFound))
			Consistently(ModuleCRExists).
				WithContext(ctx).
				WithArguments(controlPlaneClient, moduleCR.GetName(), moduleCR.GetNamespace()).
				Should(Equal(ErrNotFound))
		})

		It("Then no resources in manifest CR", func() {
			Eventually(GetManifestResource).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleName).
				Should(Equal(ErrManifestResourceIsNil))
		})

		It("Then module state of KCP Kyma in Ready", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleName, v1beta2.StateReady).
				Should(Succeed())
		})

		It("Then state of KCP kyma in Ready", func() {
			Eventually(CheckKymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})
	})

	It("When delete KCP Kyma", func() {
		Eventually(controlPlaneClient.Delete).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("Then SKR Kyma deleted")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), runtimeClient).
			Should(Succeed())
		By("Then KCP Kyma deleted")
		Eventually(KymaDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
	})
})
