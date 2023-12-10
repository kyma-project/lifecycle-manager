package e2e_test

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Manifest Skip Reconciliation Label", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given kyma deployed in KCP", func() {
		It("When enabling Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			By("Then the Module Operator is deployed on the SKR cluster")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system", "apps", "v1",
					"Deployment", runtimeClient).
				Should(Succeed())
			By("And the SKR Module Default CR is in a \"Ready\" State")
			Eventually(CheckSampleCRIsInState).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", runtimeClient, "Ready").
				Should(Succeed())
			By("And the KCP Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})

		It("When the Manifest is labelled to skip reconciliation", func() {
			labels, err := GetManifestLabels(ctx, kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient)
			Expect(err).ToNot(HaveOccurred())
			labels[declarativev2.SkipReconcileLabel] = "true"
			err = SetManifestLabels(ctx, kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient, labels)
			Expect(err).ToNot(HaveOccurred())

			By("When deleting the SKR Default CR")
			Eventually(DeleteCRWithGVK).
				WithContext(ctx).
				WithArguments(runtimeClient, "sample-yaml", "kyma-system", "operator.kyma-project.io",
					"v1alpha1", "Sample").
				Should(Succeed())
			By("Then SKR Module Default CR is not recreated")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io",
					"v1alpha1", "Sample", runtimeClient).
				ShouldNot(Succeed())

			By("When deleting the SKR Module Manager Deployment")
			err = DeleteCRWithGVK(ctx, runtimeClient, "template-operator-controller-manager",
				"template-operator-system", "apps", "v1", "Deployment")
			Expect(err).ToNot(HaveOccurred())
			By("Then Module Manager Deployment is not recreated on the SKR cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager", "template-operator-system",
					"apps", "v1", "Deployment", runtimeClient).
				ShouldNot(Succeed())
		})

		It("When the Manifest skip reconciliation label removed", func() {
			labels, err := GetManifestLabels(ctx, kyma.GetName(), kyma.GetNamespace(),
				module.Name, controlPlaneClient)
			Expect(err).ToNot(HaveOccurred())
			delete(labels, declarativev2.SkipReconcileLabel)
			err = SetManifestLabels(ctx, kyma.GetName(), kyma.GetNamespace(), module.Name,
				controlPlaneClient, labels)
			Expect(err).ToNot(HaveOccurred())

			By("Then Module Default CR is recreated")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("sample-yaml", "kyma-system",
					"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
				Should(Succeed())
			By("Then Module Deployment is recreated")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments("template-operator-controller-manager",
					"template-operator-system", "apps", "v1", "Deployment", runtimeClient).
				Should(Succeed())

			By("And the KCP Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())

			By("And the SKR Kyma CR is in a \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(defaultRemoteKymaName, remoteNamespace, runtimeClient, shared.StateReady).
				Should(Succeed())
		})
	})
})
