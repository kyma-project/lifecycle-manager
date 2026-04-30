package e2e_test

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Restricted Default Module Catalog Sync", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given a restricted default module with nil kymaSelector", func() {
		It("Then MRM and MT are NOT synced to SKR", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, ControlPlaneNamespace, kcpClient).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())

			By("MRM does NOT exist on SKR")
			Consistently(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			By("MT does NOT exist on SKR")
			Consistently(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Equal(ErrNotFound))
		})
	})

	Context("Given the MRM kymaSelector is updated to an empty selector", func() {
		It("When kymaSelector is set to empty (matches nothing)", func() {
			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace,
					&apimetav1.LabelSelector{}).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT are NOT synced to SKR", func() {
			Consistently(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			Consistently(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Equal(ErrNotFound))
		})
	})

	Context("Given the MRM kymaSelector is updated to match the Kyma labels", func() {
		It("When kymaSelector matches the Kyma", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace,
					"operator.kyma-project.io/restricted-module", "allowed").
				Should(Succeed())

			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace,
					&apimetav1.LabelSelector{
						MatchLabels: map[string]string{
							"operator.kyma-project.io/restricted-module": "allowed",
						},
					}).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT are synced to SKR", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Succeed())
		})
	})

	Context("Given the MRM kymaSelector is updated but Kyma still matches", func() {
		It("When kymaSelector is updated to a different label that still matches", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace,
					"operator.kyma-project.io/plan", "premium").
				Should(Succeed())

			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace,
					&apimetav1.LabelSelector{
						MatchLabels: map[string]string{
							"operator.kyma-project.io/plan": "premium",
						},
					}).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT remain synced to SKR", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Succeed())
		})
	})

	Context("Given the MRM kymaSelector is updated so Kyma no longer matches", func() {
		It("When kymaSelector requires a label the Kyma does not have", func() {
			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace,
					&apimetav1.LabelSelector{
						MatchLabels: map[string]string{
							"operator.kyma-project.io/plan": "enterprise",
						},
					}).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT are removed from SKR", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Equal(ErrNotFound))
		})
	})

	Context("Given the Kyma labels are updated so it matches again", func() {
		It("When the Kyma label is updated to match the selector", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace,
					"operator.kyma-project.io/plan", "enterprise").
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT are synced to SKR again", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Succeed())
		})
	})

	Context("Given the MRM kymaSelector is updated again so Kyma no longer matches", func() {
		It("When kymaSelector requires a label the Kyma does not have", func() {
			Eventually(UpdateModuleReleaseMetaKymaSelector).
				WithContext(ctx).
				WithArguments(kcpClient, module.Name, ControlPlaneNamespace,
					&apimetav1.LabelSelector{
						MatchLabels: map[string]string{
							"operator.kyma-project.io/region": "eu-west-1",
						},
					}).
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT are removed from SKR", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Equal(ErrNotFound))

			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Equal(ErrNotFound))
		})
	})

	Context("Given the MRM kymaSelector is updated to match the Kyma again", func() {
		It("When Kyma is labelled to match the new selector", func() {
			Eventually(UpdateKymaLabel).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace,
					"operator.kyma-project.io/region", "eu-west-1").
				Should(Succeed())

			Eventually(ImmediatelyRequeueKyma).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.Name, kyma.Namespace).
				Should(Succeed())
		})

		It("Then MRM and MT are synced to SKR again", func() {
			Eventually(ModuleReleaseMetaExists).
				WithContext(ctx).
				WithArguments(module.Name, RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(ModuleTemplateExists).
				WithContext(ctx).
				WithArguments(skrClient, module, NewSKRKyma()).
				Should(Succeed())
		})
	})
})
