package control_plane_test

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/controllers"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	ErrContainsUnexpectedModules     = errors.New("kyma CR contains unexpected modules")
	ErrNotContainsExpectedCondition  = errors.New("kyma CR not contains expected condition")
	ErrNotContainsExpectedAnnotation = errors.New("kyma CR not contains expected CRD annotation")
	ErrContainsUnexpectedAnnotation  = errors.New("kyma CR contains unexpected CRD annotation")
	ErrAnnotationNotUpdated          = errors.New("kyma CR annotation not updated")
)

var _ = Describe("Kyma with remote module templates", Ordered, func() {
	kyma := NewTestKyma("remote-module-template-kyma")

	moduleInSkr := v1beta2.Module{
		ControllerName:          "manifest",
		Name:                    "test-module-in-skr",
		Channel:                 v1beta2.DefaultChannel,
		RemoteModuleTemplateRef: "test-module-in-skr",
	}
	moduleInKcp := v1beta2.Module{
		ControllerName: "manifest",
		Name:           "test-module-in-kcp",
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = []v1beta2.Module{moduleInSkr, moduleInKcp}

	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
		runtimeClient, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())
	})

	templateInKcp, err := ModuleTemplateFactory(moduleInKcp, unstructured.Unstructured{}, false, false, false)
	Expect(err).ShouldNot(HaveOccurred())
	templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, false, false, false)
	Expect(err).ShouldNot(HaveOccurred())

	It("Should create moduleInKcp template in KCP", func() {
		Eventually(controlPlaneClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(templateInKcp).
			Should(Succeed())
	})

	It("Should create moduleInSkr template in SKR", func() {
		templateInSkr.Namespace = kyma.Namespace
		Eventually(runtimeClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(templateInSkr).
			Should(Succeed())
	})

	It("Should not sync the moduleInSkr template in KCP and keep it only in SKR", func() {
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, runtimeClient, templateInSkr.Name, templateInSkr.Namespace).
			Should(Succeed())
		Consistently(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, templateInSkr.Name, templateInSkr.Namespace).
			Should(MatchError(ErrNotFound))
	})

	It("Should reconcile Manifest in KCP using remote moduleInSkr template", func() {
		Eventually(ManifestExists, Timeout, Interval).
			WithArguments(ctx, kyma, moduleInSkr, controlPlaneClient).
			Should(Succeed())
	})

	It("Should not delete the module template on SKR upon Kyma deletion", func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		Consistently(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, runtimeClient, templateInSkr.Name, templateInSkr.Namespace).
			Should(Succeed())
		Consistently(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, templateInSkr.Name, templateInSkr.Namespace).
			Should(MatchError(ErrNotFound))
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, templateInKcp).Should(Succeed())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, templateInKcp).Should(Succeed())
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, templateInSkr).Should(Succeed())
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-test-remote-skr")
	moduleInSKR := v1beta2.Module{
		Name:    "skr-remote-module",
		Channel: v1beta2.DefaultChannel,
	}
	moduleInKCP := v1beta2.Module{
		Name:    "test-module-in-kcp",
		Channel: v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = []v1beta2.Module{moduleInKCP}

	SKRTemplate, err := ModuleTemplateFactory(moduleInSKR, unstructured.Unstructured{}, false, false, false)
	Expect(err).ShouldNot(HaveOccurred())
	KCPTemplate, err := ModuleTemplateFactory(moduleInKCP, unstructured.Unstructured{}, false, false, false)
	Expect(err).ShouldNot(HaveOccurred())
	remoteKyma := &v1beta2.Kyma{}

	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controllers.DefaultRemoteSyncNamespace
	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	BeforeAll(func() {
		runtimeClient, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())
	})
	registerControlPlaneLifecycleForKyma(kyma)

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(kymaExists, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma contains global channel")
		Eventually(kymaChannelMatch, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), kyma.Spec.Channel).
			Should(Succeed())

		By("Module Template created")
		Eventually(controlPlaneClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(SKRTemplate).
			Should(Succeed())
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, SKRTemplate.Name, SKRTemplate.Namespace).
			Should(Succeed())
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, KCPTemplate.Name, KCPTemplate.Namespace).
			Should(Succeed())

		By("No module synced to remote Kyma")
		Eventually(notContainsModuleInSpec, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.Namespace, moduleInKCP.Name).
			Should(Succeed())

		By("Remote Module Catalog created")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, runtimeClient, SKRTemplate.Name, controllers.DefaultRemoteSyncNamespace).
			Should(Succeed())
		Eventually(containsModuleTemplateCondition, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), controllers.DefaultRemoteSyncNamespace).
			Should(Succeed())
		Eventually(containsModuleTemplateCondition, Timeout, Interval).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())

		By("KCP Manifest CR becomes ready")
		Eventually(UpdateManifestState, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, moduleInKCP, v1beta2.StateReady).Should(Succeed())

		By("Remote Kyma contains correct conditions for Modules and ModuleTemplates")
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(runtimeClient, v1beta2.ConditionTypeModules, string(v1beta2.ConditionReason),
				metav1.ConditionTrue, remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(runtimeClient, v1beta2.ConditionTypeModuleCatalog, string(v1beta2.ConditionReason),
				metav1.ConditionTrue, remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())

		By("Remote Kyma should contain Watcher labels and annotations")
		Eventually(watcherLabelsAnnotationsExist, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma, kyma, remoteKyma.GetNamespace()).
			Should(Succeed())
	})

	It("Enable module in SKR Kyma CR", func() {
		By("add module to remote Kyma")
		Eventually(addModuleToKyma, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace(), moduleInSKR).
			Should(Succeed())

		By("SKR module not sync back to KCP Kyma")
		Consistently(notContainsModuleInSpec, Timeout, Interval).
			WithArguments(controlPlaneClient, kyma.GetName(), kyma.GetNamespace(), moduleInSKR.Name).
			Should(Succeed())

		By("Manifest CR created in KCP")
		Eventually(ManifestExists, Timeout, Interval).
			WithArguments(ctx, kyma, moduleInSKR, controlPlaneClient).
			Should(Succeed())
		By("KCP Manifest CR becomes ready")
		Eventually(UpdateManifestState, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma, moduleInSKR, v1beta2.StateReady).Should(Succeed())

		By("Remote Kyma contains correct conditions for Modules")
		Eventually(kymaHasCondition, Timeout, Interval).
			WithArguments(runtimeClient, v1beta2.ConditionTypeModules, string(v1beta2.ConditionReason), metav1.ConditionTrue,
				remoteKyma.GetName(), remoteKyma.GetNamespace()).
			Should(Succeed())
	})

	It("Synced Module Template should get reset after changed", func() {
		By("Update SKR Module Template spec.data.spec field")
		Eventually(updateModuleTemplateSpec,
			Timeout, Interval).
			WithArguments(runtimeClient, controllers.DefaultRemoteSyncNamespace, moduleInSKR.Name, "valueUpdated").
			Should(Succeed())

		By("Expect SKR Module Template spec.data.spec field get reset")
		Eventually(expectModuleTemplateSpecGetReset, 2*Timeout, Interval).
			WithArguments(runtimeClient, controllers.DefaultRemoteSyncNamespace,
				moduleInSKR.Name, "initValue").
			Should(Succeed())
	})

	It("Remote SKR Kyma get deleted when KCP Kyma get deleted", func() {
		By("Delete KCP Kyma")
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())

		By("Expect SKR Kyma get deleted")
		Eventually(kymaExists, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), controllers.DefaultRemoteSyncNamespace).
			Should(Equal(ErrNotFound))

		By("Make sure SKR Kyma not recreated")
		Consistently(kymaExists, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), controllers.DefaultRemoteSyncNamespace).
			Should(Equal(ErrNotFound))
	})

	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})

var _ = Describe("CRDs sync to SKR and annotations updated in KCP kyma", Ordered, func() {
	kyma := NewTestKyma("kyma-test-crd-update")
	moduleInKcp := v1beta2.Module{
		ControllerName: "manifest",
		Name:           "test-module-in-kcp",
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Spec.Modules = []v1beta2.Module{moduleInKcp}

	remoteKyma := &v1beta2.Kyma{}

	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	remoteKyma.Namespace = controllers.DefaultRemoteSyncNamespace
	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	BeforeAll(func() {
		runtimeClient, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())
	})
	registerControlPlaneLifecycleForKyma(kyma)
	annotations := []string{
		"moduletemplate-skr-crd-generation",
		"moduletemplate-kcp-crd-generation",
		"kyma-skr-crd-generation",
		"kyma-kcp-crd-generation",
	}

	It("module template created", func() {
		template, err := ModuleTemplateFactory(moduleInKcp, unstructured.Unstructured{}, false, false, false)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, template).
			Should(Succeed())
	})

	It("CRDs generation annotation should exist in KCP kyma", func() {
		Eventually(func() error {
			kcpKyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}

			for _, annotation := range annotations {
				if _, ok := kcpKyma.Annotations[annotation]; !ok {
					return ErrNotContainsExpectedAnnotation
				}
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("CRDs generation annotation shouldn't exist in SKR kyma", func() {
		Eventually(func() error {
			skrKyma, err := GetKyma(ctx, runtimeClient, remoteKyma.GetName(), controllers.DefaultRemoteSyncNamespace)
			if err != nil {
				return err
			}

			for _, annotation := range annotations {
				if _, ok := skrKyma.Annotations[annotation]; ok {
					return ErrContainsUnexpectedAnnotation
				}
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("Kyma CRD should sync to SKR and annotations get updated", func() {
		var kcpKymaCrd *v1.CustomResourceDefinition
		var skrKymaCrd *v1.CustomResourceDefinition
		By("Update KCP Kyma CRD")
		Eventually(func() string {
			var err error
			kcpKymaCrd, err = updateKymaCRD(controlPlaneClient)
			if err != nil {
				return ""
			}

			return getCrdSpec(kcpKymaCrd).Properties["channel"].Description
		}, Timeout, Interval).Should(Equal("test change"))

		By("SKR Kyma CRD should be updated")
		Eventually(func() *v1.CustomResourceValidation {
			var err error
			skrKymaCrd, err = fetchCrd(runtimeClient, v1beta2.KymaKind)
			if err != nil {
				return nil
			}

			return skrKymaCrd.Spec.Versions[0].Schema
		}, Timeout, Interval).Should(Equal(kcpKymaCrd.Spec.Versions[0].Schema))

		By("Kyma CR generation annotations should be updated")
		Eventually(func() error {
			kcpKyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}

			if kcpKyma.Annotations["kyma-skr-crd-generation"] != fmt.Sprint(skrKymaCrd.Generation) {
				return ErrAnnotationNotUpdated
			}
			if kcpKyma.Annotations["kyma-kcp-crd-generation"] != fmt.Sprint(skrKymaCrd.Generation) {
				return ErrAnnotationNotUpdated
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})

	It("Should regenerate Kyma CRD in SKR when deleted", func() {
		kymaCrd, err := fetchCrd(runtimeClient, v1beta2.KymaKind)
		Expect(err).NotTo(HaveOccurred())
		Eventually(runtimeClient.Delete, Timeout, Interval).
			WithArguments(ctx, kymaCrd).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func() error {
			_, err := fetchCrd(runtimeClient, v1beta2.KymaKind)
			return err
		}, Timeout, Interval).WithContext(ctx).Should(Not(HaveOccurred()))
	})

	It("Should regenerate ModuleTemplate CRD in SKR when deleted", func() {
		moduleTemplateCrd, err := fetchCrd(runtimeClient, v1beta2.ModuleTemplateKind)
		Expect(err).NotTo(HaveOccurred())
		Eventually(runtimeClient.Delete, Timeout, Interval).
			WithArguments(ctx, moduleTemplateCrd).
			WithContext(ctx).
			Should(Succeed())

		Eventually(func() error {
			_, err := fetchCrd(runtimeClient, v1beta2.ModuleTemplateKind)
			return err
		}, Timeout, Interval).WithContext(ctx).Should(Not(HaveOccurred()))
	})

	AfterAll(func() {
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})

var _ = Describe("Kyma with remote module templates from private registries", Ordered, func() {
	kyma := NewTestKyma("remote-module-template-kyma")

	moduleInSkr := v1beta2.Module{
		ControllerName:          "manifest",
		Name:                    "test-module-in-skr",
		Channel:                 v1beta2.DefaultChannel,
		RemoteModuleTemplateRef: "test-module-in-skr",
	}
	kyma.Spec.Modules = []v1beta2.Module{moduleInSkr}

	var runtimeClient client.Client
	var runtimeEnv *envtest.Environment
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
		runtimeClient, runtimeEnv = NewSKRCluster(controlPlaneClient.Scheme())
	})

	templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, true,
		false, false)
	Expect(err).ShouldNot(HaveOccurred())

	It("Should create moduleInSkr template in SKR", func() {
		templateInSkr.Namespace = kyma.Namespace
		Eventually(runtimeClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(templateInSkr).
			Should(Succeed())
	})

	It("Kyma should be in Error state with no auth secret found error message", func() {
		Eventually(IsKymaInState, Timeout, Interval).
			WithArguments(ctx, controlPlaneClient, kyma.GetName(), v1beta2.StateError).
			Should(BeTrue())

		Eventually(func() bool {
			kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), "")
			if err == nil {
				return false
			}
			if strings.Contains(kyma.Status.Modules[0].Message, ocmextensions.ErrNoAuthSecretFound.Error()) {
				return true
			}
			return false
		}, Timeout, Interval).
			Should(BeTrue())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(runtimeClient, templateInSkr).Should(Succeed())
		Expect(runtimeEnv.Stop()).Should(Succeed())
	})
})
