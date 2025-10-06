package e2e_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/kyma-project/lifecycle-manager/tests/e2e/commontestutils"
)

var _ = Describe("Purge Metrics", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(RemoteNamespace)
	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module CR exists", func() {
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(skrClient, moduleCR).
				Should(Succeed())
		})

		It("When finalizer is added to Module CR", func() {
			Expect(AddFinalizerToModuleCR(ctx, skrClient, moduleCR, moduleCRFinalizer)).
				Should(Succeed())

			By("And KCP Kyma CR has deletion timestamp set")
			Expect(DeleteKyma(ctx, kcpClient, kyma, apimetav1.DeletePropagationBackground)).
				Should(Succeed())

			Expect(KymaHasDeletionTimestamp(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())).
				Should(BeTrue())
		})

		It("Then Purge Metrics are updated", func() {
			time.Sleep(5 * time.Second)
			Eventually(PurgeMetricsAreAsExpected).
				WithContext(ctx).
				WithArguments(float64(0), 1).
				Should(BeTrue())
		})
	})
})
