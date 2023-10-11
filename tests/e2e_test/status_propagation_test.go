package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Warning Status Propagation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	GinkgoWriter.Printf("kyma before create %v\n", kyma)
	moduleName := "template-operator"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	It("When enable Template Operator", func() {
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, "regular", runtimeClient).
			Should(Succeed())
		By("Then module state of KCP Kyma in Warning")
		Eventually(CheckModuleState).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleName, v1beta2.StateWarning).
			Should(Succeed())
		By("Then state of KCP kyma in Warning")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
			Should(Succeed())
	})

	It("When disable Template Operator", func() {
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
			Should(Succeed())
		By("Then module state of KCP in Ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
	})
})
