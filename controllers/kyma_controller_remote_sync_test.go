package controllers_test

import (
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocmv1 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/meta/v1"
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
		Skip("Skip this test at the moment, until we have an agreement how to deal with diff detection in SSA.")

		By("Remote Kyma created")
		Eventually(KymaExists, Timeout, Interval).
			WithArguments(runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace).
			Should(Succeed())

		By("CR created in kcp")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout*3, Interval).Should(Succeed())
		}

		By("No spec.module in remote Kyma")
		Eventually(func() error {
			remoteKyma, err := GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace)
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
			remoteKyma, err := GetKyma(ctx, runtimeClient, kyma.GetName(), kyma.Spec.Sync.Namespace)
			if err != nil {
				return err
			}
			if !remoteKyma.ContainsCondition(v1beta1.ConditionTypeModuleCatalog) {
				return ErrNotContainsExpectedCondition
			}
			return nil
		}, Timeout, Interval)

		unwantedLabel := ocmv1.Label{Name: "test", Value: json.RawMessage(`{"foo":"bar"}`), Version: "v1"}
		By("updating a module template in the remote cluster to simulate unwanted modification")
		Eventually(ModifyModuleTemplateSpecThroughLabels(runtimeClient, kyma, unwantedLabel, true),
			Timeout, Interval).Should(Succeed())

		By("verifying the discovered override and checking the reset label")
		Eventually(ModuleTemplatesVerifyUnwantedLabel, Timeout*3, Interval).
			WithArguments(runtimeClient, kyma, unwantedLabel, true).
			Should(Succeed())
	})
})
