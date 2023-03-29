package controllers_test

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrSpecDataMismatch          = errors.New("spec.data not match")
	ErrStatusModuleStateMismatch = errors.New("status.modules.state not match")
)

var _ = Describe("Kyma with no ModuleTemplate", Ordered, func() {
	kyma := NewTestKyma("no-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in a ready state immediately", func() {
		By("having transitioned the CR State to Ready as there are no modules")
		Eventually(IsKymaInState(ctx, controlPlaneClient, kyma.GetName(), v1beta1.StateReady),
			Timeout, Interval).Should(BeTrue())
	})
})

var _ = Describe("Kyma with deprecated Condition", Ordered, func() {
	kyma := NewTestKyma("no-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should remove deprecated conditions and add required conditions", func() {
		kyma.Status.Conditions = append(kyma.Status.Conditions, metav1.Condition{
			Type:               string(v1beta1.DeprecatedConditionTypeReady),
			Status:             metav1.ConditionFalse,
			ObservedGeneration: kyma.GetGeneration(),
			Reason:             "Deprecated",
			Message:            "Deprecated",
			LastTransitionTime: metav1.Time{Time: time.Now()},
		})
		kyma.ManagedFields = nil
		Eventually(controlPlaneClient.Patch(ctx, kyma, client.Apply,
			client.FieldOwner(v1beta1.OperatorName)), Timeout, Interval).Should(Succeed())
		By("having transitioned the CR State to Ready as there are no modules")
		Eventually(CheckKymaConditions(ctx, controlPlaneClient, kyma.GetName(),
			[]v1beta1.KymaConditionType{v1beta1.ConditionTypeModules}),
			Timeout, Interval).Should(BeTrue())
	})
})

var _ = Describe("Kyma with empty ModuleTemplate", Ordered, func() {
	kyma := NewTestKyma("empty-module-kyma")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta1.Module{
			ControllerName: "manifest",
			Name:           "example-module-name",
			Channel:        v1beta1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("should result in Kyma becoming Ready", func() {
		By("checking the state to be Processing")
		Eventually(GetKymaState, 20*time.Second, Interval).
			WithArguments(kyma.GetName()).
			Should(Equal(string(v1beta1.StateProcessing)))

		By("having created new conditions in its status")
		Eventually(GetKymaConditions(kyma.GetName()), Timeout, Interval).ShouldNot(BeEmpty())
		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1beta1.StateReady),
				20*time.Second, Interval).Should(Succeed())
		}

		By("having updated the Kyma CR state to ready")
		Eventually(GetKymaState, 20*time.Second, Interval).
			WithArguments(kyma.GetName()).
			Should(BeEquivalentTo(string(v1beta1.StateReady)))

		By("Kyma status contains expected condition")
		kymaInCluster, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(
			kymaInCluster.ContainsCondition(v1beta1.ConditionTypeModules, metav1.ConditionTrue)).To(BeTrue())
		By("Module Catalog created")
		Eventually(ModuleTemplatesExist(controlPlaneClient, kyma, false),
			10*time.Second, Interval).Should(Succeed())
		kymaInCluster, err = GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		Expect(err).ShouldNot(HaveOccurred())
		Expect(
			kymaInCluster.ContainsCondition(v1beta1.ConditionTypeModuleCatalog)).To(BeFalse())
	})
})

var _ = Describe("Kyma with multiple module CRs", Ordered, func() {
	var (
		kyma      *v1beta1.Kyma
		skrModule *v1beta1.Module
		kcpModule *v1beta1.Module
	)
	kyma = NewTestKyma("kyma-test-recreate")
	skrModule = &v1beta1.Module{
		ControllerName: "manifest", // this is a module for SKR that should be installed by module-manager
		Name:           "skr-module",
		Channel:        v1beta1.DefaultChannel,
	}
	kcpModule = &v1beta1.Module{
		ControllerName: "manifest", // this is a module for KCP that should be installed by module-manager
		Name:           "kcp-module",
		Channel:        v1beta1.DefaultChannel,
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
		kyma.Spec.Modules = []v1beta1.Module{
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
		kyma.Spec.Modules, v1beta1.Module{
			ControllerName: "manifest",
			Name:           "skr-module-update",
			Channel:        v1beta1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Manifest CR should be updated after module template changed", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1beta1.StateReady),
				Timeout, Interval).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState, 20*time.Second, Interval).
			WithArguments(kyma.GetName()).
			Should(BeEquivalentTo(string(v1beta1.StateReady)))

		By("Update Module Template spec.data.spec field")
		valueUpdated := "valueUpdated"
		Eventually(updateModuleTemplateSpecData(kyma.Name, valueUpdated), Timeout, Interval).Should(Succeed())

		By("CR updated with new value in spec.resource.spec")
		Eventually(expectManifestSpecDataEquals(kyma.Name, valueUpdated), Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Kyma skip Reconciliation", Ordered, func() {
	kyma := NewTestKyma("kyma-test-update")

	kyma.Spec.Modules = append(
		kyma.Spec.Modules, v1beta1.Module{
			ControllerName: "manifest",
			Name:           "skr-module-update",
			Channel:        v1beta1.DefaultChannel,
		})

	RegisterDefaultLifecycleForKyma(kyma)

	It("Mark Kyma as skip Reconciliation", func() {
		By("CR created")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(ModuleExists(ctx, kyma, activeModule), Timeout, Interval).Should(Succeed())
		}

		By("reacting to a change of its Modules when they are set to ready")
		for _, activeModule := range kyma.Spec.Modules {
			Eventually(UpdateModuleState(ctx, kyma, activeModule, v1beta1.StateReady),
				Timeout, Interval).Should(Succeed())
		}

		By("Kyma CR should be in Ready state")
		Eventually(GetKymaState, 20*time.Second, Interval).
			WithArguments(kyma.GetName()).
			Should(BeEquivalentTo(string(v1beta1.StateReady)))

		By("Add skip-reconciliation label to Kyma CR")
		Eventually(UpdateKymaLabel(ctx, controlPlaneClient, kyma, v1beta1.SkipReconcileLabel, "true"),
			Timeout, Interval).
			Should(Succeed())
	})

	DescribeTable("Test stop reconciliation",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When update Module Template spec.data.spec field, module should not updated",
			updateModuleTemplateSpecData(kyma.Name, "valueUpdated"),
			expectManifestSpecDataEquals(kyma.Name, "initValue")),
		Entry("When put manifest into progress, kyma spec.status.modules should not updated",
			updateAllModules(kyma.Name, v1beta1.StateProcessing),
			expectKymaStatusModules(kyma.Name, v1beta1.StateReady)),
	)
})

func expectKymaStatusModules(kymaName string, state v1beta1.State) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, moduleStatus := range createdKyma.Status.Modules {
			if moduleStatus.State != state {
				return ErrStatusModuleStateMismatch
			}
		}
		return nil
	}
}

func updateAllModules(kymaName string, state v1beta1.State) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, activeModule := range createdKyma.Spec.Modules {
			if updateModuleState(createdKyma, activeModule, state) != nil {
				return err
			}
		}
		return nil
	}
}

func updateModuleTemplateSpecData(kymaName, valueUpdated string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, activeModule := range createdKyma.Spec.Modules {
			moduleTemplate, err := GetModuleTemplate(activeModule.Name)
			Expect(err).ToNot(HaveOccurred())
			moduleTemplate.Spec.Data.Object["spec"] = map[string]any{"initKey": valueUpdated}
			err = controlPlaneClient.Update(ctx, moduleTemplate)
			Expect(err).ToNot(HaveOccurred())
		}
		return nil
	}
}

func CheckKymaConditions(ctx context.Context, kcpClient client.Client, kymaName string,
	requiredConditions []v1beta1.KymaConditionType) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaName, "")
		if err != nil || len(kymaFromCluster.Status.Conditions) != len(requiredConditions) {
			return false
		}

		for _, conditionType := range requiredConditions {
			exists := false
			for _, kymaCondition := range kymaFromCluster.Status.Conditions {
				if kymaCondition.Type == string(conditionType) {
					exists = true
					break
				}
			}
			if !exists {
				return false
			}
		}
		return true
	}
}
