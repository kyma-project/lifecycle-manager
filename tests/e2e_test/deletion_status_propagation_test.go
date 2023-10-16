package e2e_test

import (
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Warning Status Propagation After When Deletion Timestamp Not Zero", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", "regular",
		v1beta2.SyncStrategyLocalSecret)
	moduleName := "template-operator"

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	BeforeAll(func() {
		// make sure we can list Kymas to ensure CRDs have been installed
		err := controlPlaneClient.List(ctx, &v1beta2.KymaList{})
		Expect(meta.IsNoMatchError(err)).To(BeFalse())
	})

	Context("Given Template Operator", func() {
		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, "regular", runtimeClient).
				Should(Succeed())
		})
		It("Kyma should be in Warning state", func() {
			Eventually(CheckKymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
				Should(Succeed())
		})
	})

	It("Should propagate Warning state even after resource is marked for deletion", func() {
		By("Getting manifest object key")
		manifestObjectKey, err := GetManifestObjectKey(ctx, controlPlaneClient,
			kyma.GetName(), kyma.GetNamespace(), moduleName)
		Expect(err).To(Not(HaveOccurred()))
		By("Getting resource object key")
		resourceObjectKey, err := GetResourceObjectKey(ctx, controlPlaneClient,
			kyma.GetName(), kyma.GetNamespace(), moduleName)
		Expect(err).To(Not(HaveOccurred()))
		By("Adding finalizer to avoid deletion")
		Eventually(AddFinalizerToSampleResource).
			WithContext(ctx).
			WithArguments(*resourceObjectKey, runtimeClient, "e2e-finalizer").
			Should(Succeed())

		By("Disabling Template Operator")
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, moduleName, runtimeClient).
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
})
