package e2e_test

import (
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var _ = Describe("Blocking Module Deletion With Multiple Module CRs with CreateAndDelete Policy", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	var manifest *v1beta2.Manifest

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster", func() {
		It("When Kyma Module is enabled on SKR Kyma CR", func() {
			Eventually(EnableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module).
				Should(Succeed())
		})

		It("Then Module Operator is deployed on SKR cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(ModuleResourceName, TestModuleResourceNamespace, "apps", "v1",
					"Deployment", skrClient).
				Should(Succeed())
			By("And KCP Kyma CR is in \"Ready\" State")
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("When two Module CRs are created on SKR Cluster", func() {
			Eventually(CreateModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-1", RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(CreateModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-2", RemoteNamespace, skrClient).
				Should(Succeed())

			By("And Kyma Module is disabled")
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		It("Then KCP Kyma CR is in \"Processing\" State", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateProcessing).
				Should(Succeed())

			By("And Manifest CR is in \"Deleting\" State")
			Eventually(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				Should(Succeed())

			By("And Default Module CR still exists on the SKR Cluster")
			Consistently(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind),
					skrClient).Should(Succeed())

			By("And all Module Resources still exist on the SKR Cluster")
			var err error
			manifest, err = GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
				module.Name)
			Expect(err).Should(Succeed())
			for _, resource := range manifest.Status.Synced {
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Consistently(CheckIfExists).
					WithContext(ctx).
					WithArguments(resource.Name, resource.Namespace, gvk.Group, gvk.Version, gvk.Kind,
						skrClient).Should(Succeed())
			}
		})

		It("When all Module CRs except Default Module CR are deleted", func() {
			Eventually(DeleteModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-1", RemoteNamespace, skrClient).
				Should(Succeed())

			Eventually(DeleteModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-2", RemoteNamespace, skrClient).
				Should(Succeed())
		})

		It("Then Default Module CR no longer exists on SKR Cluster", func() {
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					templatev1alpha1.GroupVersion.Group, templatev1alpha1.GroupVersion.Version,
					string(templatev1alpha1.SampleKind),
					skrClient).
				Should(Equal(ErrNotFound))

			By("And all Module Resources no longer exist on SKR Cluster")
			for _, resource := range manifest.Status.Synced {
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments(resource.Name, resource.Namespace, gvk.Group, gvk.Version, gvk.Kind,
						skrClient).Should(Equal(ErrNotFound))
			}

			By("And Manifest CR is deleted")
			Eventually(NoManifestExist).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())
		})
	})
})
