package e2e_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Module Without Default CR", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module without Module Default CR is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then no resources exist in Manifest CR", func() {
			Eventually(func(g Gomega, ctx context.Context) error {
				_, err := GetManifestResource(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
					module.Name)
				return err
			}).WithContext(ctx).Should(Equal(ErrManifestResourceIsNil))

			By("And no Module CR exists")
			moduleCR := NewTestModuleCR(RemoteNamespace)
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))
			Consistently(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Equal(ErrNotFound))

			By("And Kyma Module state of KCP Kyma CR is in \"Ready\" State")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					shared.StateReady).
				Should(Succeed())

			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When a blocking finalizer is added to the Manifest CR", func() {
			Eventually(AddFinalizerToManifest).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name,
					"blocking-finalizer").
				Should(Succeed())

			By("And Manifest CR has deletion timestamp set")
			Eventually(DeleteManifest).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
				Should(Succeed())
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is deleted", func() {
			Eventually(DeleteKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma, apimetav1.DeletePropagationBackground).
				Should(Succeed())
		})

		It("Then KCP Kyma CR still exists", func() {
			Eventually(KymaExists).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace()).
				Should(Equal(ErrDeletionTimestampFound))
		})
	})
})
