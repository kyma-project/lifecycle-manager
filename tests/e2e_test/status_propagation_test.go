package e2e_test

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Warning Status Propagation", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTestModuleWithFixName("template-operator", v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given Template Operator Default CR", func() {
		It("When enable Template Operator", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
		})

		It("Then module CR exist", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
		})

		It("Then resource is defined in manifest CR", func() {
			Eventually(func(g Gomega, ctx context.Context) {
				resource, err := GetManifestResource(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
				Expect(err).ToNot(HaveOccurred())
				Expect(resource.GetName()).To(Equal(moduleCR.GetName()))
				Expect(resource.GetNamespace()).To(Equal(moduleCR.GetNamespace()))
				Expect(resource.GroupVersionKind().Version).To(Equal(moduleCR.GroupVersionKind().Version))
				Expect(resource.GroupVersionKind().Group).To(Equal(moduleCR.GroupVersionKind().Group))
				Expect(resource.GroupVersionKind().Kind).To(Equal(moduleCR.GroupVersionKind().Kind))
			}).WithContext(ctx).Should(Succeed())
		})

		It("Then module state of KCP Kyma in Warning", func() {
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name, v1beta2.StateWarning).
				Should(Succeed())
		})

		It("Then state of KCP kyma in Warning", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
				Should(Succeed())
		})
	})

	It("When disable Template Operator", func() {
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
			Should(Succeed())

		By("Then no module in KCP Kyma spec")
		Eventually(NotContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
			Should(Succeed())

		By("Then module state of KCP in Ready")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
	})

	It("When enable Template Operator again", func() {
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
			Should(Succeed())

		By("Then module in KCP Kyma spec")
		Eventually(ContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
			Should(Succeed())

		By("Then state of KCP kyma in Warning")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
			Should(Succeed())
	})

	It("When the Module is disabled with an existing finalizer", func() {
		Eventually(SetFinalizer).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
				[]string{"sample.kyma-project.io/finalizer", "blocking-finalizer"}, runtimeClient).
			Should(Succeed())

		By("Disabling module")
		Eventually(DisableModule).
			WithContext(ctx).
			WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
			Should(Succeed())

		By("Then no module in KCP Kyma spec")
		Eventually(NotContainsModuleInSpec, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module.Name).
			Should(Succeed())

		By("Then the KCP Kyma is in a \"Warning\" State")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateWarning).
			Should(Succeed())
		By("And the Module Manifest CR is in a \"Warning\" State")
		Eventually(CheckManifestIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, controlPlaneClient, v2.StateWarning).
			Should(Succeed())
	})

	It("When the blocking finalizers from the Default Module CR get removed", func() {
		Eventually(SetFinalizer).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system", "operator.kyma-project.io", "v1alpha1", "Sample",
				[]string{}, runtimeClient).
			Should(Succeed())

		By("Then the Module Default CR is removed")
		Eventually(CheckIfNotExists).
			WithContext(ctx).
			WithArguments("sample-yaml", "kyma-system",
				"operator.kyma-project.io", "v1alpha1", "Sample", runtimeClient).
			Should(Succeed())

		By("Then the Manifest CR is removed")
		Eventually(ManifestExists).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
			Should(Equal(ErrNotFound))

		By("Then module state of KCP in Ready")
		Eventually(KymaIsInState).
			WithContext(ctx).
			WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, v1beta2.StateReady).
			Should(Succeed())
	})
})
