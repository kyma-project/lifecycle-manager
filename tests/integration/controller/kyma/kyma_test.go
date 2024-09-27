package kyma_test

import (
	"context"
	"errors"
	"fmt"

	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrSpecDataMismatch       = errors.New("spec.data not match")
	ErrWrongConditions        = errors.New("conditions not correct")
	ErrWrongModulesStatus     = errors.New("modules status not correct")
	ErrWrongResourceNamespace = errors.New("resource namespace not correct")
)

var _ = Describe("Kyma with no Module", Ordered, func() {
	kyma := NewTestKyma("no-module-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in a ready state immediately", func() {
		By("having transitioned the CR State to Ready as there are no modules", func() {
			Eventually(KymaIsInState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})
	})

	It("Should contain empty status.modules", func() {
		By("containing empty status.modules", func() {
			Eventually(func() error {
				createdKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				if err != nil {
					return err
				}
				if len(createdKyma.Status.Modules) != 0 {
					return ErrWrongModulesStatus
				}
				return nil
			}, Timeout, Interval).
				Should(Succeed())
		})
	})

	It("Should contain expected Modules conditions", func() {
		By("containing Modules condition", func() {
			Eventually(func() error {
				createdKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				if err != nil {
					return err
				}
				conditions := createdKyma.Status.Conditions
				if len(conditions) != 1 {
					return ErrWrongConditions
				}
				currentCondition := conditions[0]
				expectedCondition := apimetav1.Condition{
					Type:    string(v1beta2.ConditionTypeModules),
					Status:  "True",
					Message: v1beta2.ConditionMessageModuleInReadyState,
					Reason:  string(v1beta2.ReadyConditionReason),
				}

				if currentCondition.Type != expectedCondition.Type ||
					currentCondition.Status != expectedCondition.Status ||
					currentCondition.Message != expectedCondition.Message ||
					currentCondition.Reason != expectedCondition.Reason {
					return ErrWrongConditions
				}

				return nil
			}, Timeout, Interval).
				Should(Succeed())
		})
	})
})

var _ = Describe("Kyma enable one Module", Ordered, func() {
	kyma := NewTestKyma("empty-module-kyma")
	module := NewTestModule("test-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, module)

	RegisterDefaultLifecycleForKyma(kyma)

	It("should result in Kyma becoming Ready", func() {
		By("checking the state to be Processing", func() {
			Eventually(KymaIsInState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateProcessing).
				Should(Succeed())
		})

		By("having created new conditions in its status", func() {
			Eventually(containsCondition, Timeout, Interval).
				WithArguments(kyma).Should(Succeed())
		})

		By("reacting to a change of its Modules when they are set to ready", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(UpdateManifestState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name,
						shared.StateReady).
					Should(Succeed())
			}
		})

		By("having updated the Kyma CR state to ready", func() {
			Eventually(KymaIsInState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		By("Kyma status contains expected condition", func() {
			kymaInCluster, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(
				kymaInCluster.ContainsCondition(v1beta2.ConditionTypeModules, apimetav1.ConditionTrue)).To(BeTrue())
			Expect(
				kymaInCluster.ContainsCondition(v1beta2.ConditionTypeModuleCatalog,
					apimetav1.ConditionTrue)).To(BeFalse())
		})
		By("Module Catalog created", func() {
			Eventually(AllModuleTemplatesExists, Timeout, Interval).
				WithArguments(ctx, kcpClient, kyma).
				Should(Succeed())
		})
	})

	It("Should contain expected status.modules", func() {
		By("containing expected status.modules", func() {
			Eventually(func() error {
				expectedModule := v1beta2.ModuleStatus{
					Name:    module.Name,
					State:   shared.StateReady,
					Channel: v1beta2.DefaultChannel,
					Resource: &v1beta2.TrackingObject{
						PartialMeta: v1beta2.PartialMeta{
							Namespace: RemoteNamespace,
						},
					},
				}
				createdKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				if err != nil {
					return err
				}
				modulesStatus := createdKyma.Status.Modules
				if len(modulesStatus) != 1 {
					return ErrWrongModulesStatus
				}
				template, err := GetModuleTemplate(ctx, kcpClient, module, v1beta2.DefaultChannel)
				if err != nil {
					return err
				}
				moduleStatus := modulesStatus[0]
				descriptor, err := descriptorProvider.GetDescriptor(template)
				if err != nil {
					return err
				}
				if moduleStatus.Version != descriptor.Version {
					return fmt.Errorf("version mismatch: %w", ErrWrongModulesStatus)
				}
				if moduleStatus.Name != expectedModule.Name ||
					moduleStatus.State != expectedModule.State ||
					moduleStatus.Channel != expectedModule.Channel ||
					moduleStatus.Resource.Namespace != expectedModule.Resource.Namespace {
					return ErrWrongModulesStatus
				}

				return nil
			}, Timeout, Interval).
				Should(Succeed())
		})
	})
})

var _ = Describe("Kyma enable Mandatory Module or non-existent Module Kyma.Spec.Modules", Ordered, func() {
	testCases := []struct {
		enableStatement  string
		disableStatement string
		kymaName         string
		moduleName       string
	}{
		{
			enableStatement:  "enabling one mandatory Module",
			disableStatement: "disabling mandatory Module",
			kymaName:         "mandatory-module-kyma",
			moduleName:       "mandatory-template-operator",
		},
		{
			enableStatement:  "enabling one non-existing Module",
			disableStatement: "disabling non-existent Module",
			kymaName:         "non-existing-module-kyma",
			moduleName:       "non-existent-module",
		},
	}
	for _, testCase := range testCases {
		kyma := NewTestKyma(testCase.kymaName)
		RegisterDefaultLifecycleForKyma(kyma)

		It("should result Kyma in Warning state", func() {
			By(testCase.enableStatement, func() {
				kyma.Spec.Modules = append(kyma.Spec.Modules, v1beta2.Module{
					Name: testCase.moduleName, Managed: true,
				})
				Eventually(kcpClient.Update, Timeout, Interval).
					WithContext(ctx).WithArguments(kyma).Should(Succeed())
			})
			By("checking the state to be Warning", func() {
				Eventually(KymaIsInState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateWarning).
					Should(Succeed())
			})

			By("Kyma status contains expected condition", func() {
				kymaInCluster, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(
					kymaInCluster.ContainsCondition(v1beta2.ConditionTypeModules,
						apimetav1.ConditionFalse)).To(BeTrue())
			})
		})
		It("should result Kyma in Ready state", func() {
			By(testCase.disableStatement, func() {
				kyma.Spec.Modules = []v1beta2.Module{}
				Eventually(kcpClient.Update, Timeout, Interval).
					WithContext(ctx).WithArguments(kyma).Should(Succeed())
			})
			By("checking the state to be Ready", func() {
				Eventually(KymaIsInState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})

			By("Kyma status contains expected condition", func() {
				kymaInCluster, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(
					kymaInCluster.ContainsCondition(v1beta2.ConditionTypeModules, apimetav1.ConditionTrue)).To(BeTrue())
			})
		})
	}
})

func containsCondition(kyma *v1beta2.Kyma) error {
	createdKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
	if err != nil {
		return err
	}
	conditions := createdKyma.Status.Conditions
	if len(conditions) == 0 {
		return ErrWrongConditions
	}
	return nil
}

var _ = Describe("Kyma enable multiple modules", Ordered, func() {
	var (
		kyma      *v1beta2.Kyma
		skrModule v1beta2.Module
		kcpModule v1beta2.Module
	)
	kyma = NewTestKyma("kyma-test-recreate")
	skrModule = NewTestModule("skr-module", v1beta2.DefaultChannel)
	kcpModule = NewTestModule("kcp-module", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(kyma.Spec.Modules, skrModule, kcpModule)
	RegisterDefaultLifecycleForKyma(kyma)

	It("CR should be created normally and then recreated after delete", func() {
		By("CR created", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(ManifestExists, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name).
					Should(Succeed())
			}
		})
		By("Delete CR", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(DeleteModule, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma, activeModule.Name).Should(Succeed())
			}
		})

		By("CR created again", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(ManifestExists, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name).
					Should(Succeed())
			}
		})
	})

	It("CR should be deleted after removed from kyma.spec.modules", func() {
		By("CR created", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(ManifestExists, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name).
					Should(Succeed())
			}
		})
		manifestForKcpModule, err := GetManifest(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(),
			kcpModule.Name)
		Expect(err).Should(Succeed())
		By("Remove kcp-module from kyma.spec.modules", func() {
			kyma.Spec.Modules = []v1beta2.Module{
				skrModule,
			}
			Eventually(kcpClient.Update, Timeout, Interval).
				WithContext(ctx).WithArguments(kyma).Should(Succeed())
		})

		By("kcp-module deleted", func() {
			Eventually(ManifestExistsByMetadata, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, manifestForKcpModule.Namespace, manifestForKcpModule.Name).
				Should(Equal(ErrNotFound))
		})

		By("skr-module exists", func() {
			Eventually(ManifestExists, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), skrModule.Name).
				Should(Succeed())
		})
	})

	It("Disabled module should be removed from status.modules", func() {
		Eventually(func() error {
			kyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
			if err != nil {
				return err
			}
			modulesStatus := kyma.Status.Modules
			if len(modulesStatus) != 1 {
				return ErrWrongModulesStatus
			}

			if modulesStatus[0].Name == kcpModule.Name {
				return ErrWrongModulesStatus
			}

			return nil
		}, Timeout, Interval).Should(Succeed())
	})
})

var _ = Describe("Kyma skip Reconciliation", Ordered, func() {
	kyma := NewTestKyma("kyma-test-update")
	module := NewTestModule("skr-module-update", v1beta2.DefaultChannel)
	kyma.Spec.Modules = append(
		kyma.Spec.Modules, module)

	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

	It("Should deploy ModuleTemplate", func() {
		data := builder.NewModuleCRBuilder().WithSpec(InitSpecKey, InitSpecValue).Build()
		template := builder.NewModuleTemplateBuilder().
			WithLabelModuleName(module.Name).
			WithChannel(module.Channel).
			WithModuleCR(data).
			WithOCM(compdescv2.SchemaVersion).
			WithAnnotation(shared.IsClusterScopedAnnotation, shared.EnableLabelValue).Build()
		Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
	})

	It("Mark Kyma as skip Reconciliation", func() {
		By("CR created", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(ManifestExists, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name).
					Should(Succeed())
			}
		})

		By("reacting to a change of its Modules when they are set to ready", func() {
			for _, activeModule := range kyma.Spec.Modules {
				Eventually(UpdateManifestState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), activeModule.Name,
						shared.StateReady).
					Should(Succeed())
			}
		})

		By("Kyma CR should be in Ready state", func() {
			Eventually(KymaIsInState, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		By("Add skip-reconciliation label to Kyma CR", func() {
			Eventually(UpdateKymaLabel, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), shared.SkipReconcileLabel,
					"true").
				Should(Succeed())
		})
	})

	DescribeTable("Test stop reconciliation",
		func(givenCondition func() error, expectedBehavior func() error) {
			Eventually(givenCondition, Timeout, Interval).Should(Succeed())
			Eventually(expectedBehavior, Timeout, Interval).Should(Succeed())
		},
		Entry("When update Module Template spec.data.spec field, module should not updated",
			updateKCPModuleTemplateSpecData(kyma.Name, "valueUpdated"),
			expectManifestSpecDataEquals(kyma.Name, InitSpecValue)),
		Entry("When put manifest into progress, kyma spec.status.modules should not updated",
			UpdateAllManifestState(kyma.GetName(), kyma.GetNamespace(), shared.StateProcessing),
			expectKymaStatusModules(ctx, kyma, module.Name, shared.StateReady)),
	)

	It("Stop Kyma skip Reconciliation so that it can be deleted", func() {
		Eventually(UpdateKymaLabel, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma.GetName(), kyma.GetNamespace(), shared.SkipReconcileLabel,
				"false").
			Should(Succeed())
	})
})

var _ = Describe("Kyma with managed fields not in kcp mode", Ordered, func() {
	kyma := NewTestKyma("unmanaged-kyma")
	RegisterDefaultLifecycleForKyma(kyma)

	It("Should result in a managed field with manager named 'unmanaged-kyma'", func() {
		Eventually(ContainsKymaManagerField, Timeout, Interval).
			WithArguments(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace(), shared.UnmanagedKyma).
			Should(BeTrue())
	})
})

var _ = Describe("Kyma.Spec.Status.Modules.Resource.Namespace should be empty for cluster scoped modules", Ordered,
	func() {
		kyma := NewTestKyma("kyma")
		module := NewTestModule("test-module", v1beta2.DefaultChannel)
		kyma.Spec.Modules = append(
			kyma.Spec.Modules, module)
		RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

		It("Should deploy ModuleTemplate", func() {
			for _, module := range kyma.Spec.Modules {
				template := builder.NewModuleTemplateBuilder().
					WithLabelModuleName(module.Name).
					WithChannel(module.Channel).
					WithOCM(compdescv2.SchemaVersion).
					WithAnnotation(shared.IsClusterScopedAnnotation, shared.EnableLabelValue).Build()
				Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
					WithArguments(template).
					Should(Succeed())
			}
		})

		It("expect Kyma.Spec.Status.Modules.Resource.Namespace to be empty", func() {
			Eventually(func() error {
				expectedNamespace := ""
				createdKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				if err != nil {
					return err
				}
				modulesStatus := createdKyma.Status.Modules
				if len(modulesStatus) != 1 {
					return fmt.Errorf("Status not initialized %w ", ErrWrongResourceNamespace)
				}
				if modulesStatus[0].Resource == nil {
					return fmt.Errorf("Status.Modules.Resource not initialized %w ", ErrWrongResourceNamespace)
				}
				if modulesStatus[0].Resource.Namespace != expectedNamespace {
					return ErrWrongResourceNamespace
				}

				return nil
			}, Timeout, Interval).
				Should(Succeed())
		})
	})

func expectKymaStatusModules(ctx context.Context,
	kyma *v1beta2.Kyma, moduleName string, state shared.State,
) func() error {
	return func() error {
		return CheckModuleState(ctx, kcpClient, kyma.Name, kyma.Namespace, moduleName, state)
	}
}

func updateKCPModuleTemplateSpecData(kymaName, valueUpdated string) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, kcpClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, activeModule := range createdKyma.Spec.Modules {
			return UpdateModuleTemplateSpec(ctx, kcpClient,
				activeModule, InitSpecKey, valueUpdated, createdKyma.Spec.Channel)
		}
		return nil
	}
}
