package e2e_test

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Background Kyma Deletion", Ordered, func() {
	kyma := NewKymaWithSyncLabel("kyma-sample", "kcp-system", v1beta2.DefaultChannel,
		v1beta2.SyncStrategyLocalSecret)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	moduleCR := NewTestModuleCR(remoteNamespace)

	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Cluster", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(runtimeClient, defaultRemoteKymaName, remoteNamespace, module).
				Should(Succeed())
			Eventually(ModuleCRExists).
				WithContext(ctx).
				WithArguments(runtimeClient, moduleCR).
				Should(Succeed())
			Eventually(ManifestExists).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.Name, kyma.Namespace, module.Name).
				Should(Succeed())
		})

		It("When KCP Kyma CR is deleted using Background propagation", func() {
			Eventually(DeleteKyma).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma, apimetav1.DeletePropagationBackground).
				Should(Succeed())
		})

		It("Then Manifest CR is removed before Kyma CR", func() {
			Consistently(ManifestCRIsRemovedBeforeKymaCR).
				WithContext(ctx).
				WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), module.Name).
				Should(Succeed())
		})
	})
})
