package controllers_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrContainsUnexpectedModules    = errors.New("kyma CR contains unexpected modules")
	ErrNotContainsExpectedCondition = errors.New("kyma CR not contains expected condition")
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
		Eventually(KymaExists, Timeout, Interval).
			WithArguments(runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace).
			Should(Succeed())

		By("add skr-module-client to remoteKyma.spec.modules")
		Eventually(UpdateRemoteModule(ctx, runtimeClient, kyma, []v1beta1.Module{
			*skrModuleFromClient,
		}), Timeout, Interval).Should(Succeed())

		By("skr-module-client created in kcp")
		Eventually(ModuleExists(ctx, kyma, *skrModuleFromClient), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Kyma sync into Remote Cluster", Ordered, func() {
	kyma := NewTestKyma("kyma-test-remote-skr")

	kyma.Spec.Sync = v1beta1.Sync{
		Enabled:      true,
		Strategy:     v1beta1.SyncStrategyLocalClient,
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
			if !remoteKyma.ContainsCondition(v1beta1.ConditionTypeModuleCatalog) {
				return ErrNotContainsExpectedCondition
			}
			return nil
		}, Timeout, Interval)
		By("Remote Kyma should contain Watcher labels and annotations")
		Eventually(WatcherLabelsAnnotationsExist, Timeout, Interval).
			WithArguments(runtimeClient, kyma).
			Should(Succeed())
		moduleToBeUpdated := kyma.Spec.Modules[0].Name
		By("Update SKR Module Template spec.data.spec field")
		Eventually(updateModuleTemplateSpec,
			Timeout, Interval).
			WithArguments(runtimeClient, kyma, moduleToBeUpdated, "valueUpdated", true).
			Should(Succeed())

		By("Expect SKR Module Template spec.data.spec field get reset")
		Eventually(expectModuleTemplateSpecGetReset, Timeout, Interval).
			WithArguments(runtimeClient, kyma, moduleToBeUpdated, "initValue", true).Should(Succeed())
	})
})
