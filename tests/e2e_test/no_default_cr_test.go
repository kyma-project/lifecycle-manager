package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Module Without Default CR", Ordered, func() {
	kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", "regular")

	It("Should create empty Kyma CR on remote cluster", func() {
		Eventually(CreateKymaSecret).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
		Eventually(controlPlaneClient.Create).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
		By("verifying kyma is ready")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
		By("verifying remote kyma is ready")
		Eventually(CheckRemoteKymaCR).
			WithContext(ctx).
			WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
			Should(Succeed())
	})

	It("Should enable Template Operator and Kyma should result in Ready status", func() {
		By("Enabling Template Operator")
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "regular", runtimeClient).
			Should(Succeed())
		By("Checking state of kyma")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
	})

	It("Should delete KCP Kyma", func() {
		By("Deleting KCP Kyma")
		Eventually(controlPlaneClient.Delete).
			WithContext(ctx).
			WithArguments(kyma).
			Should(Succeed())
	})

	It("Kyma CR should be removed", func() {
		Eventually(CheckKCPKymaCRDeleted).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
			Should(Succeed())
	})
})
