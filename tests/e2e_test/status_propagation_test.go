//go:build status_propagation_e2e

package e2e_test

import (
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Enable Template Operator, Kyma CR should have status `Warning`",
	Ordered, func() {
		channel := "regular"
		kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", channel)
		GinkgoWriter.Printf("kyma before create %v\n", kyma)
		remoteNamespace := "kyma-system"

		BeforeAll(func() {
			// make sure we can list Kymas to ensure CRDs have been installed
			err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
			Expect(meta.IsNoMatchError(err)).To(BeFalse())
		})

		It("Should create empty Kyma CR on remote cluster", func() {
			Eventually(CreateKymaSecret, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
			Eventually(controlPlaneClient.Create, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
			By("verifying kyma is ready")
			Eventually(CheckKymaIsInState, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
			By("verifying remote kyma is ready")
			Eventually(CheckRemoteKymaCR, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(remoteNamespace, []v1beta2.Module{}, runtimeClient, v1beta2.StateReady).
				Should(Succeed())
		})

		It("Should enable Template Operator and Kyma should result in Warning status", func() {
			By("Enabling Template Operator")
			Eventually(EnableModule, timeout, interval).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "regular", runtimeClient).
				Should(Succeed())
			By("Checking state of kyma")
			Eventually(CheckKymaIsInState, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
				Should(Succeed())
		})

		It("Should disable Template Operator and Kyma should result in Ready status", func() {
			By("Disabling Template Operator")
			Eventually(DisableModule, timeout, interval).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
				Should(Succeed())
			By("Checking state of kyma")
			Eventually(CheckKymaIsInState, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
				Should(Succeed())
		})

		It("Should delete KCP Kyma", func() {
			By("Deleting KCP Kyma")
			Eventually(controlPlaneClient.Delete, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma).
				Should(Succeed())
		})

		It("Kyma CR should be removed", func() {
			Eventually(CheckKCPKymaCRDeleted, timeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient).
				Should(Succeed())
		})

	})
