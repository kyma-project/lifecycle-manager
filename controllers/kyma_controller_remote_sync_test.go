package controllers_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrContainsUnexpectedModules    = errors.New("kyma CR contains unexpected modules")
	ErrNotContainsExpectedCondition = errors.New("kyma CR not contains expected condition")
)

var _ = Describe("Kyma with multiple module CRs in remote sync mode", Ordered, func() {
	var kyma *v1beta2.Kyma
	var skrModule *v1beta2.Module
	skrModuleFromClient := &v1beta2.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync-client",
		Channel:        v1beta2.DefaultChannel,
	}
	kyma = NewTestKyma("kyma-remote-sync")
	skrModule = &v1beta2.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync",
		Channel:        v1beta2.DefaultChannel,
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
		Skip("TODO: revisit it after 542 merged")
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithArguments(runtimeClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())

		By("add skr-module-client to remoteKyma.spec.modules")
		Eventually(UpdateRemoteModule(ctx, runtimeClient, kyma, []v1beta2.Module{
			*skrModuleFromClient,
		}), Timeout, Interval).Should(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ModuleExists(ctx, kyma, *skrModuleFromClient), Timeout, Interval).Should(Succeed())
	})
})

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

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           "skr-remote-module",
			Channel:        v1beta2.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Kyma CR should be synchronized in both clusters", func() {
		Skip("TODO: revisit it after 542 merged")
		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithArguments(runtimeClient, kyma.GetName(), kyma.GetNamespace()).
			Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("No spec.module in remote Kyma")
		Eventually(func() error {
			remoteKyma, err := GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}
			if len(remoteKyma.Spec.Modules) != 0 {
				return ErrContainsUnexpectedModules
			}
			return nil
		}, Timeout, Interval)

		By("Remote Module Catalog created")
		Eventually(ModuleTemplatesExist(runtimeClient, kyma, true), Timeout, Interval).Should(Succeed())
		Eventually(func() error {
			remoteKyma, err := GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}
			if !remoteKyma.ContainsCondition(v1beta2.ConditionTypeModuleCatalog) {
				return ErrNotContainsExpectedCondition
			}
			return nil
		}, Timeout, Interval)
		moduleToBeUpdated := kyma.Spec.Modules[0].Name
		By("Update SKR Module Template spec.data.spec field")
		Eventually(updateModuleTemplateSpec,
			Timeout, Interval).
			WithArguments(runtimeClient, kyma, moduleToBeUpdated, "valueUpdated").
			Should(Succeed())

		By("Expect SKR Module Template spec.data.spec field get reset")
		Eventually(expectModuleTemplateSpecGetReset, Timeout, Interval).
			WithArguments(runtimeClient, kyma, moduleToBeUpdated, "initValue").Should(Succeed())
	})
})
