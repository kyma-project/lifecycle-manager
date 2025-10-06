package e2e_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

var _ = Describe("Misconfigured Kyma Secret", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	BeforeAll(func() {
		By("When a KCP Kyma CR is created on the KCP cluster with misconfigured Kyma Secret")
		Eventually(CreateInvalidKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient).
			Should(Succeed())
		Eventually(kcpClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
	})
	CleanupKymaAfterAll(kyma)

	Context("Given Two Cluster Setup", func() {
		It("When Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module).
				Should(Succeed())
		})

		It("Then Module is not created", func() {
			Consistently(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Not(Succeed()))
			By("No Manifest CR exists")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())
		})

		It("When Kyma Secret is corrected", func() {
			Eventually(DeleteAccessSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(Succeed())
			Consistently(AccessSecretExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(MatchError(accessmanager.ErrAccessSecretNotFound))
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), string(*skrConfig)).
				Should(Succeed())
		})

		It("Then Module is created", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
			By("Manifest CR is in \"Ready\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateReady).
				Should(Succeed())
		})
	})
})
