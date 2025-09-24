package e2e_test

import (
	"fmt"

	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ModuleTemplate without ModuleReleaseMeta is not synced", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	var skrKyma *v1beta2.Kyma
	oldFormatModuleName := "old-format-module"
	version := "1.0.0-test"

	InitEmptyKymaBeforeAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma can be fetched from SKR Cluster", func() {
			var err error
			skrKyma, err = GetKyma(ctx, skrClient, shared.DefaultRemoteKymaName, RemoteNamespace)
			if err != nil {
				Fail("Failed to get SKR Kyma")
			}
		})

		It("When an old format ModuleTemplate is created without ModuleReleaseMeta in KCP", func() {
			// Create an old format ModuleTemplate (with Channel and no ModuleReleaseMeta)
			oldFormatModuleTemplate := builder.NewModuleTemplateBuilder().
				WithName(fmt.Sprintf("%s-%s", oldFormatModuleName, version)).
				WithChannel(v1beta2.DefaultChannel).
				WithLabelModuleName(oldFormatModuleName).
				WithOCM(compdescv2.SchemaVersion).
				Build()

			Expect(CreateModuleTemplate(ctx, kcpClient, oldFormatModuleTemplate)).To(Succeed())

			By("Then the old format ModuleTemplate exists in the KCP Cluster")
			Eventually(func() error {
				return ModuleTemplateExists(ctx, kcpClient, v1beta2.Module{Name: oldFormatModuleName, Channel: v1beta2.DefaultChannel}, kyma)
			}).Should(Succeed())

			By("But the old format ModuleTemplate should NOT be synced to the SKR Cluster")
			Consistently(func() error {
				return ModuleTemplateExists(ctx, skrClient, v1beta2.Module{Name: oldFormatModuleName, Channel: v1beta2.DefaultChannel}, skrKyma)
			}).ShouldNot(Succeed())
		})

		It("When enabling the old format module in Kyma spec", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, v1beta2.Module{Name: oldFormatModuleName, Channel: v1beta2.DefaultChannel}).
				Should(Succeed())

			By("Then the module should remain in Warning state due to missing ModuleReleaseMeta")
			Eventually(CheckModuleState).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, oldFormatModuleName, shared.StateWarning).
				Should(Succeed())

			By("And no Manifest should be created for the old format module")
			Consistently(func() error {
				return ManifestExists(ctx, kcpClient, kyma.Name, kyma.Namespace, oldFormatModuleName)
			}).ShouldNot(Succeed())
		})

		It("Cleanup: Remove the old format ModuleTemplate and module from Kyma", func() {
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace, oldFormatModuleName).
				Should(Succeed())

			Eventually(DeleteModuleTemplate).
				WithContext(ctx).
				WithArguments(kcpClient, v1beta2.Module{Name: oldFormatModuleName, Channel: v1beta2.DefaultChannel}, kyma).
				Should(Succeed())
		})
	})
})
