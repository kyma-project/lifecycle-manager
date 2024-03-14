package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var _ = Describe("Misconfigured Kyma Secret", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)

	Context("Given Working Two Cluster Setup", func() {
		It("When Kyma Secret is misconfigured and Module enabled", func() {
			Eventually(DeleteKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(CreateInvalidKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())

			By("And Module is enabled")
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module).
				Should(Succeed())
		})

		It("Then Module is not created", func() {
			Consistently(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Not(Succeed()))

			By("And Manifest CR is in \"Error\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateError).
				Should(Succeed())
		})

		It("When Kyma Secret is corrected", func() {
			Eventually(DeleteKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

		It("Then Module is created", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
			By("Manifest CR is in \"Ready\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient,
					shared.StateReady).
				Should(Succeed())

			By("And Kyma Module will be disabled")
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})
	})
})
