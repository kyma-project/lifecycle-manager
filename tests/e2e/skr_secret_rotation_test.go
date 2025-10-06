package e2e_test

import (
	"os"
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SKR client cache get evicted due to connection error caused by secret rotation", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	testSKRAdmin := "alice"

	Context("Given a new SKR admin user exists", func() {
		It("When configuring Kyma Secret with the SKR admin kubeconfig, then the secret is created and bound", func() {
			cmd := exec.CommandContext(ctx, "kubectl", "config", "use-context", "k3d-skr")
			_, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.CommandContext(ctx, "sh", "../../scripts/tests/create-new-cluster-admin.sh", testSKRAdmin)
			output, err := cmd.CombinedOutput()
			GinkgoWriter.Printf("Create new SKR admin user: %s\n", output)
			Expect(err).NotTo(HaveOccurred())

			By("Then grant the SKR admin cluster-admin privileges")
			cmd = exec.CommandContext(ctx, "kubectl", "create", "clusterrolebinding", testSKRAdmin+"-cluster-admin",
				"--clusterrole=cluster-admin", "--user="+testSKRAdmin)
			_, err = cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())

			By("And create the Kyma Secret using the test SKR admin kubeconfig")
			testSKRAdminKubeconfigFile := testSKRAdmin + "-kubeconfig.yaml"
			runtimeConfig, err := os.ReadFile(testSKRAdminKubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), string(runtimeConfig)).
				Should(Succeed())
		})
	})

	Context("When Kyma is set up using the test SKR admin", func() {
		It("Then the Kyma CR becomes Ready on KCP and SKR and runtime watcher is running", func() {
			Eventually(kcpClient.Create).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("Then the Kyma CR is in a Ready state on the KCP cluster")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			By("And the Kyma CR is in a Ready state on the SKR cluster")
			Eventually(CheckRemoteKymaCR).
				WithContext(ctx).
				WithArguments(RemoteNamespace, []v1beta2.Module{}, skrClient, shared.StateReady).
				Should(Succeed())
			By("And the Runtime Watcher deployment on SKR is up and running", func() {
				Eventually(DeploymentIsReady).
					WithContext(ctx).
					WithArguments(skrClient, skrwebhookresources.SkrResourceName,
						RemoteNamespace).
					Should(Succeed())
			})
		})
	})

	Context("Given the SKR cluster is available", func() {
		It("When a Kyma Module is enabled on SKR, then the Module CR is created", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
		})

		It("Then the KCP Kyma CR remains in Ready state", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("And the module status on KCP reports Ready", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateReady).
				Should(Succeed())
		})
	})

	Context("When reducing SKR admin user permissions", func() {
		It("When removing the cluster-admin binding, then the admin privileges are revoked", func() {
			cmd := exec.CommandContext(ctx, "kubectl", "delete", "clusterrolebinding", testSKRAdmin+"-cluster-admin")
			_, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
		})
		It("Then grant only view privileges to the SKR admin user", func() {
			cmd := exec.CommandContext(ctx, "kubectl", "create", "clusterrolebinding", testSKRAdmin+"-view",
				"--clusterrole=view", "--user="+testSKRAdmin)
			_, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Then KCP Kyma CR and Manifest CR should enter Error state due to reduced permissions", func() {
		It("Then the KCP Kyma CR transitions to Error and stays there", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateError).
				Should(Succeed())

			By("And the Manifest CR reports Error state")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateError).
				Should(Succeed())
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateError).
				Should(Succeed())
		})
	})

	Context("When updating the Kyma Secret to the default cluster admin kubeconfig", func() {
		It("When replacing the secret, then the SKR client cache should renew and controllers recover", func() {
			By("Delete the existing Kyma Secret")
			Eventually(DeleteAccessSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(Succeed())
			Consistently(AccessSecretExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName()).
				Should(MatchError(accessmanager.ErrAccessSecretNotFound))
			By("Create a new Kyma Secret with the default admin kubeconfig")
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), string(*skrConfig)).
				Should(Succeed())
		})
	})

	Context("Then KCP Kyma CR and Manifest CR should return to Ready state", func() {
		It("Then the KCP Kyma CR becomes Ready again and remains stable", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			Consistently(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			By("And the Manifest CR is Ready and remains stable")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateReady).
				Should(Succeed())
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateReady).
				Should(Succeed())
		})
	})
})
