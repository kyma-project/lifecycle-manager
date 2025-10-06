package e2e_test

import (
	"os/exec"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

func RunDeletionTest(deletionPropagation apimetav1.DeletionPropagation) {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Cluster", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())

			By("And finalizer is added to Module CR")
			Expect(AddFinalizerToModuleCR(ctx, skrClient, moduleCR, moduleCRFinalizer)).
				Should(Succeed())

			By("And Kyma Module is disabled")
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then Module CR stays in \"Deleting\" State", func() {
			Eventually(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR, shared.StateDeleting).
				Should(BeTrue())
			Consistently(ModuleCRIsInExpectedState).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR, shared.StateDeleting).
				Should(BeTrue())

			By("And Manifest CR is in \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("When KCP Kyma CR is deleted", func() {
			Eventually(DeleteKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma, deletionPropagation).
				Should(Succeed())
		})

		It("Then KCP Kyma CR still exists", func() {
			Eventually(KymaExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace()).
				Should(Equal(ErrDeletionTimestampFound))
		})

		It("When SKR Cluster is removed", func() {
			cmd := exec.CommandContext(ctx, "sh", "../../scripts/tests/remove_skr_host_from_coredns.sh")
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))
			cmd = exec.CommandContext(ctx, "k3d", "cluster", "rm", "skr")
			out, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))
		})

		It("Then KCP Kyma CR is in \"Error\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())
		})

		It("When Kubeconfig Secret is deleted", func() {
			Consistently(AccessSecretExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(Succeed())
			Eventually(DeleteAccessSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(Succeed())
			Consistently(AccessSecretExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(MatchError(accessmanager.ErrAccessSecretNotFound))
		})

		It("Then Manifest CR is deleted", func() {
			Eventually(ManifestExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
				Should(Equal(ErrNotFound))

			By("And KCP Kyma CR is deleted")
			Eventually(KymaDeleted).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient).
				Should(Succeed())
		})
	})
}
