package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Module Upgrade", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system",
		v1beta2.DefaultChannel, v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)
	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given an SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})

		It("And Module Operator Deployment exists", func() {
			Eventually(ModuleDeploymentExists).
				WithContext(ctx).
				WithArguments(runtimeClient, "template-operator-system", "template-operator-v1-controller-manager").
				Should(BeTrue())
		})

		It("And KCP Kyma CR is in \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the Module Distribution Channel is changed", func() {
			Eventually(UpdateKymaModuleChannel).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, "fast").
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})

		It("And new Module Operator Deployment exists", func() {
			Eventually(ModuleDeploymentExists).
				WithContext(ctx).
				WithArguments(runtimeClient, "template-operator-system", "template-operator-v2-controller-manager").
				Should(BeTrue())
		})

		It("And old Module Operator Deployment does not exist", func() {
			Eventually(ModuleDeploymentExists).
				WithContext(ctx).
				WithArguments(runtimeClient, "template-operator-system", "template-operator-v1-controller-manager").
				Should(BeFalse())
		})

		It("And KCP Kyma CR is in \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
