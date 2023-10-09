package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
)

var _ = Describe("Non Blocking Module Deletion", Ordered, func() {
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

	It("Should disable Template Operator with existing blocking finalizer "+
		"and Kyma should result in Processing status", func() {
		By("Inserting finalizer into Sample CR")
		Eventually(SetFinalizer).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io/v1alpha1", "Sample",
				[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
			Should(Succeed())
		By("Disabling Template Operator")
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
			Should(Succeed())
		By("Checking state of kyma set to `Processing`")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateProcessing).
			Should(Succeed())
		By("Sample CR should not be removed")
		Eventually(CheckIfExists).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io/v1alpha1", "Sample",
				runtimeClient).
			Should(Succeed())
		By("Checking template-operator deployment still exists")
		Eventually(CheckIfExists).
			WithContext(ctx).
			WithArguments("template-operator-controller-manager", "template-operator-system", "apps/v1",
				"Deployment", runtimeClient).
			Should(Succeed())
		By("Checking if it is possible to update Sample CR")
		Eventually(UpdateSampleCRSpec).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "newValue", runtimeClient).
			Should(Succeed())
	})

	It("Should be able to re-enable module under deletion", func() {
		By("Re-Enabling Template Operator")
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", "regular", runtimeClient).
			Should(Succeed())
		By("Removing all finalizers from sample CR")
		Eventually(SetFinalizer).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io/v1alpha1", "Sample",
				[]string{}, runtimeClient).
			Should(Succeed())
		By("New Sample CR should exists")
		Eventually(CheckIfExists).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io/v1alpha1", "Sample",
				runtimeClient).
			Should(Succeed())
		By("Checking state of kyma set to `Ready`")
		Eventually(CheckKymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateProcessing).
			Should(Succeed())
	})

	It("Should remove all finalizers from Sample CR, all resources from template-operator should be removed",
		func() {
			By("Disabling Template Operator")
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, "template-operator", runtimeClient).
				Should(Succeed())
			By("Sample CR should  be removed")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io/v1alpha1", "Sample",
					runtimeClient).
				Should(Not(Succeed()))
			By("Checking template-operator deployment still exists")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system", "apps/v1",
					"Deployment", runtimeClient).
				Should(Not(Succeed()))
			By("Checking state of kyma set to `Ready`")
			Eventually(CheckKymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateProcessing).
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
