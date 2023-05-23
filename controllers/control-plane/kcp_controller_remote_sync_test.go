package control_plane_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/controllers"
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
	var skrModule v1beta2.Module
	skrModuleFromClient := v1beta2.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync-client",
		Channel:        v1beta2.DefaultChannel,
	}
	kyma = NewTestKyma("kyma-remote-sync")
	skrModule = v1beta2.Module{
		ControllerName: "manifest",
		Name:           "skr-module-sync",
		Channel:        v1beta2.DefaultChannel,
	}
	kyma.Labels[v1beta2.SyncLabel] = v1beta2.EnableLabelValue

	kyma.Spec.Modules = append(kyma.Spec.Modules, skrModule)
	remoteKyma := kyma
	remoteKyma.Name = v1beta2.DefaultRemoteKymaName

	registerControlPlaneLifecycleForKyma(kyma)

	It("module template created", func() {
		template, err := ModuleTemplateFactory(skrModuleFromClient, unstructured.Unstructured{}, false)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, template).
			Should(Succeed())
	})

	It("CR add from client should be synced in both clusters", func() {
		By("Remote Kyma created")
		Eventually(kymaExists, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), controllers.DefaultRemoteSyncNamespace).
			Should(Succeed())

		By("add skr-module-client to remoteKyma.spec.modules")
		Eventually(updateRemoteModule(ctx, runtimeClient, remoteKyma, controllers.DefaultRemoteSyncNamespace, []v1beta2.Module{
			skrModuleFromClient,
		}), Timeout, Interval).Should(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ManifestExists, Timeout, Interval).WithArguments(ctx, kyma,
			skrModuleFromClient, controlPlaneClient).Should(Succeed())
	})
})

var _ = Describe("Kyma with remote module templates", Ordered, func() {
	kyma := NewTestKyma("remote-module-template-kyma")
	kyma.Labels[v1beta2.SyncLabel] = v1beta2.EnableLabelValue

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

	templateInKcp, err := ModuleTemplateFactory(moduleInKcp, unstructured.Unstructured{}, false)
	Expect(err).ShouldNot(HaveOccurred())
	templateInSkr, err := ModuleTemplateFactory(moduleInSkr, unstructured.Unstructured{}, false)
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
	})
})

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-test-remote-skr")
	kyma.Labels[v1beta2.SyncLabel] = v1beta2.EnableLabelValue

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta2.Module{
			ControllerName: "manifest",
			Name:           "skr-remote-module",
			Channel:        v1beta2.DefaultChannel,
		})

	remoteKyma := kyma
	remoteKyma.Name = v1beta2.DefaultRemoteKymaName
	registerControlPlaneLifecycleForKyma(kyma)

	It("Kyma CR should be synchronized in both clusters", func() {
		By("Remote Kyma created")
		Eventually(kymaExists, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma.GetName(), controllers.DefaultRemoteSyncNamespace).
			Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ManifestExists, Timeout, Interval).WithArguments(
				ctx, kyma, activeModule, controlPlaneClient).Should(Succeed())
		}

		By("No spec.module in remote Kyma")
		Eventually(func() error {
			remoteKyma, err := GetKyma(ctx, runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace())
			if err != nil {
				return err
			}
			if len(remoteKyma.Spec.Modules) != 0 {
				return ErrContainsUnexpectedModules
			}
			return nil
		}, Timeout, Interval)

		By("Remote Module Catalog created")
		Eventually(ModuleTemplatesExist(ctx, runtimeClient, kyma, controllers.DefaultRemoteSyncNamespace),
			Timeout, Interval).
			Should(Succeed())
		Eventually(func() error {
			remoteKyma, err := GetKyma(ctx, runtimeClient, remoteKyma.GetName(), remoteKyma.GetNamespace())
			if err != nil {
				return err
			}
			if !remoteKyma.ContainsCondition(v1beta2.ConditionTypeModuleCatalog) {
				return ErrNotContainsExpectedCondition
			}
			return nil
		}, Timeout, Interval)

		By("Remote Kyma should contain Watcher labels and annotations")
		Eventually(watcherLabelsAnnotationsExist, Timeout, Interval).
			WithArguments(runtimeClient, remoteKyma, controllers.DefaultRemoteSyncNamespace).
			Should(Succeed())
		moduleToBeUpdated := remoteKyma.Spec.Modules[0].Name

		By("Update SKR Module Template spec.data.spec field")
		Eventually(updateModuleTemplateSpec,
			Timeout, Interval).
			WithArguments(runtimeClient, controllers.DefaultRemoteSyncNamespace, moduleToBeUpdated, "valueUpdated").
			Should(Succeed())

		By("Expect SKR Module Template spec.data.spec field get reset")
		Eventually(expectModuleTemplateSpecGetReset, Timeout, Interval).
			WithArguments(runtimeClient, controllers.DefaultRemoteSyncNamespace,
				moduleToBeUpdated, "initValue").
			Should(Succeed())
	})
})
