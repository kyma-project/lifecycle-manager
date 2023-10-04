package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Module Without Default CR", Ordered, func() {
	kyma := NewKymaForE2E("kyma-sample", "kcp-system", "regular")
	moduleName := "template-operator"

	It("When create empty Kyma CR on remote cluster", func() {
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

	It("When enable Template Operator", func() {
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, "regular", runtimeClient).
			Should(Succeed())
		By("Then module state of KCP Kyma in Ready")
		Eventually(CheckModuleState).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleName, v1beta2.StateReady).
			Should(Succeed())

		By("Then state of KCP kyma in Ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
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
