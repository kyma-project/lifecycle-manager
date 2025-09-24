package e2e_test

import (
	"os"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("SKR client cache get evicted due to connection error caused by secret rotation", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)

	testSKRAdmin := "alice"

	Context("Create new SKR admin user", func() {
		It("Based on k3d-skr context", func() {
			cmd := exec.CommandContext(ctx, "kubectl", "config", "use-context", "k3d-skr")
			_, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.CommandContext(ctx, "sh", "../../scripts/tests/create-new-cluster-admin.sh", testSKRAdmin)
			output, err := cmd.CombinedOutput()
			GinkgoWriter.Printf("Create new SKR admin user: %s\n", output)
			Expect(err).NotTo(HaveOccurred())

			By("Create kyma secret with test skr admin kubeconfig")
			testSKRAdminKubeconfigPath := testSKRAdmin + "-kubeconfig.yaml"
			runtimeConfig, err := os.ReadFile(testSKRAdminKubeconfigPath)
			Eventually(CreateKymaSecret).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), string(runtimeConfig)).
				Should(Succeed())
		})
	})

	Context("Setup Kyma with test skr admin", func() {
		It("Create kyma CR", func() {
			Eventually(kcpClient.Create).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("Then the Kyma CR is in a \"Ready\" State on the KCP cluster ")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
			By("And the Kyma CR is in \"Ready\" State on the SKR cluster")
			Eventually(CheckRemoteKymaCR).
				WithContext(ctx).
				WithArguments(RemoteNamespace, []v1beta2.Module{}, skrClient, shared.StateReady).
				Should(Succeed())
			By("And Runtime Watcher deployment is up and running in SKR", func() {
				Eventually(CheckPodLogs).
					WithContext(ctx).
					WithArguments(RemoteNamespace, skrwebhookresources.SkrResourceName, "server",
						"Starting server for validation endpoint", skrRESTConfig,
						skrClient, &apimetav1.Time{Time: time.Now().Add(-5 * time.Minute)}).
					Should(Succeed())
			})
		})
	})

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
		})

		It("Then KCP Kyma CR is in \"Ready\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("And KCP Kyma CR status.modules are in \"Ready\" State", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name, shared.StateReady).
				Should(Succeed())
		})
	})
})
