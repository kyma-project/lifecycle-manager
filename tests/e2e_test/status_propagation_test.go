package e2e_test

import (
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Warning Status Propagation", Ordered, func() {
	kyma := testutils.NewKymaForE2E("kyma-sample", "kcp-system", "regular")
	GinkgoWriter.Printf("kyma before create %v\n", kyma)

	BeforeAll(func() {
		// make sure we can list Kymas to ensure CRDs have been installed
		err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
		Expect(meta.IsNoMatchError(err)).To(BeFalse())
	})

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

	It("Should enable Template Operator and Kyma should result in Warning status", func() {
		By("Enabling Template Operator")
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "regular", runtimeClient).
			Should(Succeed())
		By("Checking state of kyma")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
			Should(Succeed())
	})

	It("Should disable Template Operator and Kyma should result in Ready status", func() {
		By("Disabling Template Operator")
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
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

var _ = Describe("Resource State 'Warning' should be propagated to Kyma CR Module Status",
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
			By("Checking module state in Kyma CR")
			Eventually(CheckKymaModuleIsInState, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient,
					"template-operator", v1beta2.StateWarning).
				Should(Succeed())
		})

		It("should propagate Warning state even after marked for deletion", func() {
			By("Getting manifest object key")
			manifestObjectKey, err := GetManifestObjectKey(ctx, controlPlaneClient,
				kyma.GetName(), "template-operator")
			Expect(err).To(Not(HaveOccurred()))
			By("Adding finalizer to avoid deletion")
			Eventually(AddFinalizerToManifest, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(manifestObjectKey, controlPlaneClient, "e2e-finalizer").
				Should(Succeed())

			By("Mark Manifest for deletion")
			Eventually(DeleteManifest, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(manifestObjectKey, controlPlaneClient).
				Should(Succeed())

			By("Checking module state in Kyma CR")
			Eventually(CheckKymaModuleIsInState, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient,
					"template-operator", v1beta2.StateWarning).
				Should(Succeed())

			By("Remove finalizer")
			Eventually(RemoveFinalizerToManifest, statusTimeout, interval).
				WithContext(ctx).
				WithArguments(manifestObjectKey, controlPlaneClient, "e2e-finalizer").
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
