package e2e_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	templatev1alpha1 "github.com/kyma-project/template-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// testModuleDeletionBlocking is a shared helper function that parametrizes module deletion blocking tests.
// It verifies that lifecycle-manager properly blocks module deletion when user-created Module CRs exist,
// regardless of the CustomResourcePolicy or the namespace where CRs are located.
func testModuleDeletionBlocking(
	kyma *v1beta2.Kyma,
	module *v1beta2.Module,
	userCreatedCRs []types.NamespacedName,
	addFinalizerToDefaultCR bool,
) {
	var manifest *v1beta2.Manifest
	expectDefaultCR := module.CustomResourcePolicy == v1beta2.CustomResourcePolicyCreateAndDelete

	It("When Kyma Module is enabled on SKR Kyma CR", func() {
		Eventually(EnableModule).
			WithContext(ctx).
			WithArguments(skrClient, defaultRemoteKymaName, RemoteNamespace, *module).
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

		if expectDefaultCR {
			By("And Default Module CR exists (CreateAndDelete policy)")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace, templatev1alpha1.GroupVersion.Group,
					templatev1alpha1.GroupVersion.Version, string(templatev1alpha1.SampleKind),
					skrClient).Should(Succeed())
		}
	})

	if addFinalizerToDefaultCR {
		It("When finalizer is added to default Module CR", func() {
			By("Adding finalizer to default Module CR")
			defaultCRNamespacedName := types.NamespacedName{
				Name:      TestModuleCRName,
				Namespace: RemoteNamespace,
			}
			moduleCR := &unstructured.Unstructured{}
			moduleCR.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   templatev1alpha1.GroupVersion.Group,
				Version: templatev1alpha1.GroupVersion.Version,
				Kind:    string(templatev1alpha1.SampleKind),
			})
			Expect(skrClient.Get(ctx, defaultCRNamespacedName, moduleCR)).To(Succeed())
			Expect(AddFinalizerToModuleCR(ctx, skrClient, moduleCR, "test-finalizer")).To(Succeed())
		})
	} else if len(userCreatedCRs) > 0 {
		It("When user-created Module CRs are created", func() {
			for _, cr := range userCreatedCRs {
				By("Creating Module CR: " + cr.Name + " in namespace: " + cr.Namespace)
				Eventually(CreateModuleCR).
					WithContext(ctx).
					WithArguments(cr.Name, cr.Namespace, skrClient).
					Should(Succeed())
			}
		})
	}

	It("When Kyma Module is disabled", func() {
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

		By("And all Module Resources still exist on the SKR Cluster")
		var err error
		manifest, err = GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), module.Name)
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

	switch {
	case len(userCreatedCRs) > 1:
		It("When one user-created CR is deleted", func() {
			Eventually(DeleteModuleCR).
				WithContext(ctx).
				WithArguments(userCreatedCRs[0].Name, userCreatedCRs[0].Namespace, skrClient).
				Should(Succeed())
		})

		It("Then deletion is still blocked due to remaining CRs", func() {
			By("Verifying Manifest CR is still in \"Deleting\" State")
			Consistently(CheckManifestIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), module.Name, kcpClient,
					shared.StateDeleting).
				WithTimeout(3 * time.Second).
				Should(Succeed())

			By("And Module Resources still exist")
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

		It("When remaining user-created CRs are deleted", func() {
			for _, cr := range userCreatedCRs[1:] {
				Eventually(DeleteModuleCR).
					WithContext(ctx).
					WithArguments(cr.Name, cr.Namespace, skrClient).
					Should(Succeed())
			}
		})
	case len(userCreatedCRs) == 1:
		It("When the user-created CR is deleted", func() {
			Eventually(DeleteModuleCR).
				WithContext(ctx).
				WithArguments(userCreatedCRs[0].Name, userCreatedCRs[0].Namespace, skrClient).
				Should(Succeed())
		})
	case addFinalizerToDefaultCR:
		It("When finalizer is removed from default Module CR", func() {
			By("Removing finalizer from default Module CR")
			defaultCRNamespacedName := types.NamespacedName{
				Name:      TestModuleCRName,
				Namespace: RemoteNamespace,
			}
			moduleCR := &unstructured.Unstructured{}
			moduleCR.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   templatev1alpha1.GroupVersion.Group,
				Version: templatev1alpha1.GroupVersion.Version,
				Kind:    string(templatev1alpha1.SampleKind),
			})
			Expect(skrClient.Get(ctx, defaultCRNamespacedName, moduleCR)).To(Succeed())
			moduleCR.SetFinalizers([]string{})
			Expect(skrClient.Update(ctx, moduleCR)).To(Succeed())
		})
	}

	It("Then module deletion is unblocked", func() {
		if expectDefaultCR {
			By("Verifying Default Module CR is automatically deleted (CreateAndDelete policy)")
			Eventually(CheckIfExists).
				WithContext(ctx).
				WithArguments(TestModuleCRName, RemoteNamespace,
					templatev1alpha1.GroupVersion.Group, templatev1alpha1.GroupVersion.Version,
					string(templatev1alpha1.SampleKind),
					skrClient).
				Should(Equal(ErrNotFound))
		}

		By("And all Module Resources are deleted")
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
}
