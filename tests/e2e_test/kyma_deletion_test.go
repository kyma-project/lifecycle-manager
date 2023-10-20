package e2e_test

import (
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KCP Kyma CR Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	moduleCR := NewTestModuleCR(remoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	BeforeAll(func() {
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), moduleName, kyma.Spec.Channel, controlPlaneClient).
			Should(Succeed())
		Eventually(ModuleCRExists).
			WithContext(ctx).
			WithArguments(runtimeClient, moduleCR).
			Should(Succeed())
	})

	Context("Given kyma deployed in KCP with Template Operator enabled", func() {
		It("When adding finalizer to Sample CR", func() {
			Expect(AddFinalizerToModuleCR(ctx, runtimeClient, moduleCR, moduleCRFinalizer)).
				Should(Succeed())
		})

		It("And disabling Template Operator", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), moduleName, controlPlaneClient).
				Should(Succeed())
		})

		It("Then Module CR should be stuck in Deleting state", func() {
			Eventually(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR, v1beta2.StateDeleting).
				Should(BeTrue())
			Consistently(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR, v1beta2.StateDeleting).
				Should(BeTrue())
		})

		It("When Kyma in KCP cluster is deleted", func() {
			Eventually(DeleteKyma).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma).
				Should(Succeed())
		})

		It("Then KCP Kyma should still exist", func() {
			Expect(KymaExists(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())).Should(BeTrue())
		})

		It("When SKR Cluster is removed", func() {
			cmd := exec.Command("k3d", "cluster", "rm", "skr")
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))
		})

		It("Then KCP Kyma should be in Error state", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateError).
				Should(BeTrue())
		})

		It("When KCP Kyma secret is deleted", func() {
			Eventually(DeleteKymaSecret).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

		It("Then KCP Kyma should be deleted", func() {
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})
	})
})
