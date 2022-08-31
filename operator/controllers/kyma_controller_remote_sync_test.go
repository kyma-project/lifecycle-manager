package controllers_test

import (
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	var kyma *v1alpha1.Kyma
	moduleTemplates := make(map[string]*v1alpha1.ModuleTemplate)
	var skrModule *v1alpha1.Module
	skrModuleFromClient := &v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync-client",
		Channel:        v1alpha1.ChannelStable,
	}
	BeforeAll(func() {
		kyma = NewTestKyma("kyma-remote-sync")
		skrModule = &v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "skr-module-sync",
			Channel:        v1alpha1.ChannelStable,
		}

		kyma.Spec.Sync = v1alpha1.Sync{
			Enabled:      true,
			Strategy:     v1alpha1.SyncStrategyLocalClient,
			Namespace:    namespace,
			NoModuleCopy: true,
		}
		kyma.Spec.Modules = append(kyma.Spec.Modules, *skrModule)
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(ctx, client.ObjectKey{Name: kyma.Name, Namespace: namespace}, kyma)).Should(Succeed())
	})

	It("module template created", func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates[module.Name] = template
		}
		template, err := test.ModuleTemplateFactory(*skrModuleFromClient, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
		moduleTemplates[skrModuleFromClient.Name] = template
	})

	It("CR add from client should be synced in both clusters", func() {
		By("Remote Kyma created")
		Eventually(RemoteKymaExists(runtimeClient, kyma.GetName()), 30*time.Second, interval).Should(Succeed())
		remoteKyma, err := GetKyma(runtimeClient, kyma.GetName())
		Expect(err).ShouldNot(HaveOccurred())

		By("add skr-module-client to remoteKyma.spec.modules")
		remoteKyma.Spec.Modules = []v1alpha1.Module{
			*skrModuleFromClient,
		}
		Expect(runtimeClient.Update(ctx, remoteKyma.SetObservedGeneration())).To(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ModuleExists(controlPlaneClient, kyma.GetName(), moduleTemplates[skrModuleFromClient.Name]), timeout, interval).Should(BeTrue())
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})
})

var _ = Describe("Kyma sync into Remote Cluster", func() {
	kyma := NewTestKyma("kyma-test-remote-skr")

	kyma.Spec.Sync = v1alpha1.Sync{
		Enabled:      true,
		Strategy:     v1alpha1.SyncStrategyLocalClient,
		Namespace:    namespace,
		NoModuleCopy: true,
	}

	RegisterDefaultLifecycleForKyma(kyma)

	kyma.Spec.Modules = append(kyma.Spec.Modules, v1alpha1.Module{
		ControllerName: "manifest",
		Name:           "skr-remote-module",
		Channel:        v1alpha1.ChannelStable,
	})

	moduleTemplates := make([]*v1alpha1.ModuleTemplate, 0)

	BeforeEach(func() {
		for _, module := range kyma.Spec.Modules {
			template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(controlPlaneClient.Create(ctx, template)).To(Succeed())
			moduleTemplates = append(moduleTemplates, template)
		}
	})

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(RemoteKymaExists(runtimeClient, kyma.GetName()), 30*time.Second, interval).Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range moduleTemplates {
			Eventually(ModuleExists(controlPlaneClient, kyma.GetName(), activeModule), timeout, interval).Should(BeTrue())
		}

		By("No spec.module in remote Kyma")
		remoteKyma, err := GetKyma(runtimeClient, kyma.GetName())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(remoteKyma.Spec.Modules).To(BeEmpty())

		By("Remote Module Catalog created")
		Eventually(CatalogExists(runtimeClient, kyma), 30*time.Second, interval).Should(Succeed())
	})
})
