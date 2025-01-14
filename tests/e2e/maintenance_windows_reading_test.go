package e2e_test

import (
	"os/exec"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Reading Maintenance Window Policy", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)

	Context("Given KCP Cluster With No Maintenance Window File", func() {
		It("Then maintenance-config ConfigMap does not exist", func() {
			Eventually(MaintenanceWindowConfigMapExists).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Equal(ErrConfigMapNotExist))
		})

		It("When maintenance windows are applied", func() {
			cmd := exec.Command("(", "cd", "../../config/watcher_local_test", "&&", "kustomize", "edit", "add",
				"component",
				"../maintenance_windows", ")")
			out, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf(string(out))
		})

		It("Then maintenance-config ConfigMap exists in the KCP Cluster", func() {
			Eventually(MaintenanceWindowConfigMapExists).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())

			By("And the data read from the ConfigMap is correct")
		})
	})
})
