package e2e_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This test validates the fix for https://github.com/kyma-project/lifecycle-manager/issues/3006
// It ensures that lifecycle-manager checks ALL Module CRs across all namespaces before
// proceeding with module deletion, as required by ADR https://github.com/kyma-project/community/issues/972
var _ = Describe("Blocking Module Deletion With Module CRs in Different Namespaces", Ordered, func() {
	kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
	module := NewTemplateOperator(v1beta2.DefaultChannel)
	module.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
	var manifest *v1beta2.Manifest
	const crossNamespace = "default" // Using 'default' namespace for the cross-namespace CR

	InitEmptyKymaBeforeAll(kyma)
	CleanupKymaAfterAll(kyma)

	Context("Given SKR Cluster with module enabled", func() {
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

		It("When Module CRs are created in different namespaces", func() {
			By("Creating a Module CR in the default namespace (kyma-system)")
			Eventually(CreateModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-in-kyma-system", RemoteNamespace, skrClient).
				Should(Succeed())

			By("Creating a Module CR in a different namespace (default)")
			Eventually(CreateModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-in-default-ns", crossNamespace, skrClient).
				Should(Succeed())

			By("And Kyma Module is disabled")
			Eventually(DisableModule).
				WithContext(ctx).
				WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
				Should(Succeed())
		})

		//nolint:dupl // Intentional duplication for test clarity and independence
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
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments(resource.Name, resource.Namespace, gvk.Group, gvk.Version, gvk.Kind,
						skrClient).Should(Succeed())
			}
		})

		It("When the Module CR in the default namespace (kyma-system) is deleted", func() {
			Eventually(DeleteModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-in-kyma-system", RemoteNamespace, skrClient).
				Should(Succeed())
		})

		It("Then Module deletion is still blocked because CR exists in different namespace", func() {
			By("Verifying Manifest CR is still in \"Deleting\" State")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				WithTimeout(3 * time.Second).
				Should(Succeed())

			By("And Module Resources still exist on the SKR Cluster")
			for _, resource := range manifest.Status.Synced {
				gvk := schema.GroupVersionKind{
					Group:   resource.Group,
					Version: resource.Version,
					Kind:    resource.Kind,
				}
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments(resource.Name, resource.Namespace, gvk.Group, gvk.Version, gvk.Kind,
						skrClient).Should(Succeed())
			}
		})

		It("When the Module CR in the cross-namespace (default) is also deleted", func() {
			Eventually(DeleteModuleCR).
				WithContext(ctx).
				WithArguments("sample-cr-in-default-ns", crossNamespace, skrClient).
				Should(Succeed())
		})

		It("Then all Module Resources no longer exist on SKR Cluster", func() {
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

var _ = Describe(
	"Blocking Module Deletion With Module CRs in Different Namespaces (CreateAndDelete Policy)",
	Ordered,
	func() {
		kyma := NewKymaWithNamespaceName("kyma-sample", ControlPlaneNamespace, v1beta2.DefaultChannel)
		module := NewTemplateOperator(v1beta2.DefaultChannel)
		module.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete
		var manifest *v1beta2.Manifest
		const crossNamespace = "default" // Using 'default' namespace for the cross-namespace CR

		InitEmptyKymaBeforeAll(kyma)
		CleanupKymaAfterAll(kyma)

		Context("Given SKR Cluster with module enabled", func() {
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

				By("And Default Module CR exists in kyma-system namespace")
				Eventually(CheckIfExists).
					WithContext(ctx).
					WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
						templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind),
						skrClient).Should(Succeed())
			})

			It("When an additional Module CR is created in a different namespace (default)", func() {
				By("Creating a Module CR in the default namespace")
				Eventually(CreateModuleCR).
					WithContext(ctx).
					WithArguments("sample-cr-in-default-ns", crossNamespace, skrClient).
					Should(Succeed())

				By("And Kyma Module is disabled")
				Eventually(DisableModule).
					WithContext(ctx).
					WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, module.Name).
					Should(Succeed())
			})

			//nolint:dupl // Intentional duplication for test clarity and independence
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

				By("And Default Module CR still exists in kyma-system namespace")
				Consistently(CheckIfExists).
					WithContext(ctx).
					WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
						templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind),
				skrClient).
				WithTimeout(3 * time.Second).
				Should(Succeed())

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

			It("When the Module CR in the cross-namespace (default) is deleted", func() {
				Eventually(DeleteModuleCR).
					WithContext(ctx).
					WithArguments("sample-cr-in-default-ns", crossNamespace, skrClient).
					Should(Succeed())
			})

			It("Then Default Module CR and all Module Resources are deleted", func() {
				By("Verifying Default Module CR no longer exists")
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
