package controllers_test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Kyma with no ModuleTemplate", Ordered, func() {
	kyma := NewTestKyma("no-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in a ready state immediately", func() {
		By("having transitioned the CR State to Ready as there are no modules")
		Eventually(IsKymaInState(ctx, controlPlaneClient, kyma.GetName(), v1alpha1.StateReady),
			Timeout, Interval).Should(BeTrue())
	})
})

var _ = Describe("Kyma with empty ModuleTemplate", Ordered, func() {
	kyma := NewTestKyma("empty-module-kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1alpha1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("should result in Kyma becoming Ready", func() {
		By("checking the state to be Processing")
		Eventually(GetKymaState(kyma.GetName()), 20*time.Second, Interval).
			Should(BeEquivalentTo(string(v1alpha1.StateProcessing)))

		By("having created new conditions in its status")
		Eventually(GetKymaConditions(kyma.GetName()), Timeout, Interval).ShouldNot(BeEmpty())
		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1alpha1.StateReady),
				20*time.Second, Interval).Should(Succeed())
		}

		By("having updated the Kyma CR state to ready")
		Eventually(GetKymaState(kyma.GetName()), 20*time.Second, Interval).
			Should(BeEquivalentTo(string(v1alpha1.StateReady)))

		By("Kyma status contains expected condition")
		kymaInCluster, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(
			kymaInCluster.ContainsCondition(
				v1alpha1.ConditionTypeReady,
				v1alpha1.ConditionReasonModulesAreReady, metav1.ConditionTrue)).To(BeTrue())
		By("Module Catalog created")
		Eventually(ModuleTemplatesExist(controlPlaneClient, kyma, false), 10*time.Second, Interval).Should(Succeed())
		kymaInCluster, err = GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(
			kymaInCluster.ContainsCondition(
				v1alpha1.ConditionTypeReady,
				v1alpha1.ConditionReasonModuleCatalogIsReady,
			)).To(BeFalse())
	})
})

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	var (
		kyma      *v1alpha1.Kyma
		skrModule *v1alpha1.Module
		kcpModule *v1alpha1.Module
	)
	kyma = NewTestKyma("kyma-test-recreate")
	skrModule = &v1alpha1.Module{
		ControllerName: "manifest", // this is a module for SKR that should be installed by module-manager
		Name:           "skr-module",
		Channel:        v1alpha1.DefaultChannel,
	}
	kcpModule = &v1alpha1.Module{
		ControllerName: "manifest", // this is a module for KCP that should be installed by module-manager
		Name:           "kcp-module",
		Channel:        v1alpha1.DefaultChannel,
	}
	kyma.Spec.Modules = append(kyma.Spec.Modules, *skrModule, *kcpModule)
	RegisterDefaultLifecycleForKyma(kyma)

	It("CR should be created normally and then recreated after delete", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}
		By("Delete CR")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(deleteModule(kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("CR created again")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}
	})

	It("CR should be deleted after removed from kyma.spec.modules", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}
		By("Remove kcp-module from kyma.spec.modules")
		kyma.Spec.Modules = []v1alpha1.Module{
			*skrModule,
		}
		Expect(controlPlaneClient.Update(ctx, kyma)).To(Succeed())

		By("kcp-module deleted")
		Eventually(ModuleNotExist(ctx, kyma, *kcpModule), Timeout, Interval).Should(Succeed())

		By("skr-module exists")
		Eventually(ModuleExists(ctx, kyma, *skrModule), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Kyma update Manifest CR", Ordered, func() {
	kyma := NewTestKyma("kyma-test-update")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "skr-module-update",
			Channel:        v1alpha1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1alpha1.StateReady),
				Timeout, Interval).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState(kyma.GetName()), Timeout, Interval).
			Should(BeEquivalentTo(string(v1alpha1.StateReady)))

		By("Add skip-reconciliation label to Kyma CR")
		Eventually(UpdateKymaLabel(ctx, controlPlaneClient, kyma, v1alpha1.SkipReconcileLabel, "true"),
			Timeout, Interval).
			Should(BeEquivalentTo(string(v1alpha1.StateReady)))

		By("Update Module Template spec.data.spec field")
		valueUpdated := "valueUpdated"
		for _, activeModule := range kyma.Spec.Modules {
			moduleTemplate, err := GetModuleTemplate(activeModule.Name)
			Expect(err).ToNot(HaveOccurred())
			moduleTemplate.Spec.Data.Object["spec"] = map[string]any{"initKey": valueUpdated}
			err = controlPlaneClient.Update(ctx, moduleTemplate)
			Expect(err).ToNot(HaveOccurred())
		}
		By("CR updated with new value in spec.resource.spec")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(SKRModuleExistWithOverwrites(kyma, activeModule),
				Timeout, Interval).Should(Equal(valueUpdated))
		}
	})
})

var _ = Describe("Kyma skip Reconciliation", Ordered, func() {
	kyma := NewTestKyma("kyma-test-update")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1alpha1.Module{
			ControllerName: "manifest",
			Name:           "skr-module-update",
			Channel:        v1alpha1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should stop updated after kyma marked with skip label", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1alpha1.StateReady),
				20*time.Second, Interval).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState(kyma.GetName()), 20*time.Second, Interval).
			Should(BeEquivalentTo(string(v1alpha1.StateReady)))

		By("Update Module Template spec.data.spec field")
		valueUpdated := "valueUpdated"
		for _, activeModule := range kyma.Spec.Modules {
			moduleTemplate, err := GetModuleTemplate(activeModule.Name)
			Expect(err).ToNot(HaveOccurred())
			moduleTemplate.Spec.Data.Object["spec"] = map[string]any{"initKey": valueUpdated}
			err = controlPlaneClient.Update(ctx, moduleTemplate)
			Expect(err).ToNot(HaveOccurred())
		}
		By("CR updated with new value in spec.resource.spec")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(SKRModuleExistWithOverwrites(kyma, activeModule),
				Timeout, Interval).Should(Equal(valueUpdated))
		}
	})
})
