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
	moduleCR := NewTestModuleCR(remoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given Template Operator Default CR", func() {
		It("When enable Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, kyma.Spec.Channel, runtimeClient).
				Should(Succeed())
		})

		It("Then module CR exist", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(controlPlaneClient, moduleCR.GetName(), moduleCR.GetNamespace()).
				Should(Succeed())
		})

		It("Then resource defined in manifest CR", func() {
			Eventually(GetManifestResource).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleName).
				Should(Equal(moduleCR))
		})

		It("Then module state of KCP Kyma in Warning", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleName, v1beta2.StateWarning).
				Should(Succeed())
		})

		It("Then state of KCP kyma in Warning", func() {
			Eventually(CheckKymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
				Should(Succeed())
		})
	})

	It("When disable Template Operator", func() {
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, runtimeClient).
			Should(Succeed())
		By("Then module state of KCP in Ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
	})
})
