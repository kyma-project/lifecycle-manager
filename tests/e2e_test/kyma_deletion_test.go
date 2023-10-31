package e2e_test

import (
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("KCP Kyma CR Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)

	InitEmptyKymaBeforeAll(kyma)

	Context("Given kyma deployed in KCP", func() {
		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})

		It("And adding finalizer to Sample CR", func() {
			Expect(AddFinalizerToModuleCR(ctx, runtimeClient, moduleCR, moduleCRFinalizer)).
				Should(Succeed())
		})

		It("And disabling Template Operator", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then Module CR should be stuck in Deleting state", func() {
			Eventually(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR, shared.StateDeleting).
				Should(BeTrue())
			Consistently(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR, shared.StateDeleting).
				Should(BeTrue())
		})

		It("When Kyma in KCP cluster is deleted", func() {
			Eventually(DeleteKyma).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma).
				Should(Succeed())
		})

		It("Then KCP Kyma should still exist", func() {
			Expect(KymaExists(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())).
				Should(Equal(ErrDeletionTimestampFound))
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
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateError).
				Should(Succeed())
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
