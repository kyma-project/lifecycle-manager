package e2e_test

import (
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Warning Status Propagation After When Deletion Timestamp Not Zero", Ordered, func() {
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

	It("Should propagate Warning state even after resource is marked for deletion", func() {
		By("Getting manifest object key")
		manifestObjectKey, err := GetManifestObjectKey(ctx, controlPlaneClient,
			kyma.GetName(), kyma.GetNamespace(), "template-operator")
		Expect(err).To(Not(HaveOccurred()))
		By("Getting resource object key")
		resourceObjectKey, err := GetResourceObjectKey(ctx, controlPlaneClient,
			kyma.GetName(), kyma.GetNamespace(), "template-operator")
		Expect(err).To(Not(HaveOccurred()))
		By("Adding finalizer to avoid deletion")
		Eventually(AddFinalizerToSampleResource).
			WithContext(ctx).
			WithArguments(*resourceObjectKey, runtimeClient, "e2e-finalizer").
			Should(Succeed())

		By("Disabling Template Operator")
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
			Should(Succeed())

		By("Checking manifest deletion timestamp")
		Eventually(CheckManifestDeletionTimestamp).
			WithContext(ctx).
			WithArguments(*manifestObjectKey, controlPlaneClient, false).
			Should(Succeed())
		By("Checking manifest state")
		Eventually(CheckManifestIsInState).
			WithContext(ctx).
			WithArguments(*manifestObjectKey, controlPlaneClient, declarative.StateWarning).
			Should(Succeed())
		By("Checking module state in Kyma CR")
		Eventually(CheckKymaModuleIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient,
				"template-operator", v1beta2.StateWarning).
			Should(Succeed())

		By("Remove finalizer")
		err = RemoveFinalizerToSampleResource(ctx, *resourceObjectKey, runtimeClient, "e2e-finalizer")
		Expect(err).To(Not(HaveOccurred()))
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
