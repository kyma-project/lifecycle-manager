package controllers_test

import (
	"encoding/json"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocmv1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	var kyma *v1beta1.Kyma
	var skrModule *v1beta1.Module
	skrModuleFromClient := &v1beta1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync-client",
		Channel:        v1beta1.DefaultChannel,
	}
	kyma = NewTestKyma("kyma-remote-sync")
	skrModule = &v1beta1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync",
		Channel:        v1beta1.DefaultChannel,
	}

	kyma.Spec.Sync = v1beta1.Sync{
		Enabled:      true,
		Strategy:     v1beta1.SyncStrategyLocalClient,
		Namespace:    metav1.NamespaceDefault,
		NoModuleCopy: true,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, *skrModule)

	RegisterDefaultLifecycleForKyma(kyma)

	It("module template created", func() {
		template, err := ModuleTemplateFactory(*skrModuleFromClient, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(controlPlaneClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
	})

	It("CR add from client should be synced in both clusters", func() {
		By("Remote Kyma created")
		Eventually(KymaExists(runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace), 30*time.Second, Interval).
			Should(Succeed())

		By("add skr-module-client to remoteKyma.spec.modules")
		Eventually(UpdateRemoteModule(ctx, runtimeClient, kyma, []v1beta1.Module{
			*skrModuleFromClient,
		}), Timeout, Interval).Should(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ModuleExists(ctx, kyma, *skrModuleFromClient), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Kyma with remote module templates", Ordered, func() {
	kyma := NewTestKyma("remote-module-template-kyma")
	kyma.Spec.Sync = v1beta1.Sync{
		Enabled:      true,
		Strategy:     v1beta1.SyncStrategyLocalClient,
		Namespace:    metav1.NamespaceDefault,
		NoModuleCopy: true,
	}
	moduleInSkr := v1beta1.Module{
		ControllerName:          "manifest",
		Name:                    "test-module-in-skr",
		Channel:                 v1beta1.DefaultChannel,
		RemoteModuleTemplateRef: "test-module-in-skr",
	}
	moduleInKcp := v1beta1.Module{
		ControllerName: "manifest",
		Name:           "test-module-in-kcp",
		Channel:        v1beta1.DefaultChannel,
	}
	kyma.Spec.Modules = []v1beta1.Module{moduleInSkr, moduleInKcp}

	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})

	It("Should create moduleInKcp template in KCP", func() {
		templateInKcp, err := ModuleTemplateFactory(moduleInKcp, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, templateInKcp)).To(Succeed())
	})

	It("Should create moduleInSkr template in SKR", func() {
		templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		// Async Assertion because it takes time for the ModuleTemplate CRD to be installed on the remote cluster
		Eventually(runtimeClient.Create(ctx, templateInSkr), Timeout, Interval).
			Should(Succeed())
	})

	It("Should not sync the moduleInSkr template in KCP and keep it only in SKR", func() {
		Eventually(ModuleTemplatesExist(runtimeClient, kyma, true), Timeout, Interval).
			Should(Succeed())

		templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(ModuleTemplateExist(controlPlaneClient, kyma, templateInSkr), Timeout, Interval).Should(BeTrue())
	})

	It("Should reconcile Manifest in KCP using remote moduleInSkr template", func() {
		Eventually(ModuleExists(ctx, kyma, moduleInSkr), Timeout, Interval).
			Should(Succeed())
	})

	It("Should not delete the module template on SKR upon Kyma deletion", func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())

		templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(getModuleTemplate(runtimeClient, templateInSkr, kyma, true)).To(Succeed())
	})

	AfterAll(func() {
		templateInKcp, err := ModuleTemplateFactory(moduleInKcp, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(controlPlaneClient.Delete(ctx, templateInKcp)).To(Succeed())
		Expect(runtimeClient.Delete(ctx, templateInKcp)).To(Succeed())
		Expect(runtimeClient.Delete(ctx, templateInSkr)).To(Succeed())
	})
})

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-test-remote-skr")

	kyma.Spec.Sync = v1beta1.Sync{
		Enabled:      true,
		Strategy:     v1beta1.SyncStrategyLocalClient,
		Namespace:    "sync-namespace",
		NoModuleCopy: true,
	}

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta1.Module{
			ControllerName: "manifest",
			Name:           "skr-remote-module",
			Channel:        v1beta1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(KymaExists(runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace), 30*time.Second, Interval).
			Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("No spec.module in remote Kyma")
		remoteKyma, err := GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(remoteKyma.Spec.Modules).To(BeEmpty())

		By("Remote Module Catalog created")
		Eventually(ModuleTemplatesExist(runtimeClient, kyma, true), Timeout, Interval).Should(Succeed())
		Eventually(func() {
			remoteKyma, err = GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(remoteKyma.ContainsCondition(v1beta1.ConditionTypeModuleCatalog)).To(BeTrue())
		}, Timeout, Interval)

		unwantedLabel := ocmv1.Label{Name: "test", Value: json.RawMessage(`{"foo":"bar"}`), Version: "v1"}
		By("updating a module template in the remote cluster to simulate unwanted modification")
		Eventually(ModifyModuleTemplateSpecThroughLabels(runtimeClient, kyma, unwantedLabel, true),
			Timeout, Interval).Should(Succeed())

		By("verifying the discovered override and checking the reset label")
		Eventually(ModuleTemplatesVerifyUnwantedLabel(
			runtimeClient, kyma, unwantedLabel, true), Timeout, Interval).Should(Succeed())
	})
})
