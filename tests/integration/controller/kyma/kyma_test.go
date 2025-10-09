package kyma_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

var (
	ErrSpecDataMismatch       = errors.New("spec.data not match")
	ErrWrongResourceNamespace = errors.New("resource namespace not correct")
)

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
		skrKyma := NewSKRKyma()
		var skrClient client.Client
		var err error

		BeforeAll(func() {
			Eventually(CreateCR, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma).Should(Succeed())
			Eventually(func() error {
				skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
				return err
			}, Timeout, Interval).Should(Succeed())
		})
		AfterAll(func() {
			Eventually(DeleteCR, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma).Should(Succeed())
		})

		BeforeEach(func() {
			Eventually(SyncKyma, Timeout, Interval).
				WithContext(ctx).WithArguments(skrClient, skrKyma).Should(Succeed())
		})

		It("should result Kyma in Warning state", func() {
			By(testCase.enableStatement, func() {
				module := v1beta2.Module{
					Name: testCase.moduleName, Managed: true,
				}
				Eventually(EnableModule, Timeout, Interval).
					WithContext(ctx).
					WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace(), module).
					Should(Succeed())
			})
			By("checking the state to be Warning in KCP", func() {
				Eventually(KymaIsInState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateWarning).
					Should(Succeed())
			})
			By("checking the state to be Warning in SKR", func() {
				Eventually(KymaIsInState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(skrKyma.GetName(), skrKyma.GetNamespace(), skrClient, shared.StateWarning).
					Should(Succeed())
			})
			By("Kyma status contains expected condition in KCP", func() {
				kymaInCluster, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(
					kymaInCluster.ContainsCondition(v1beta2.ConditionTypeModules,
						apimetav1.ConditionFalse)).To(BeTrue())
			})
			By("Kyma status contains expected condition in SKR", func() {
				kymaInCluster, err := GetKyma(ctx, skrClient, skrKyma.GetName(), skrKyma.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(
					kymaInCluster.ContainsCondition(v1beta2.ConditionTypeModules,
						apimetav1.ConditionFalse)).To(BeTrue())
			})
		})
		It("should result Kyma in Ready state", func() {
			By(testCase.disableStatement, func() {
				kymaUpdateFunc := func(skrKyma *v1beta2.Kyma) error {
					skrKyma.Spec.Modules = []v1beta2.Module{}
					return nil
				}
				Eventually(UpdateKymaWithFunc, Timeout, Interval).
				WithContext(ctx).
				WithArguments(skrClient, skrKyma.GetName(), skrKyma.GetNamespace(), kymaUpdateFunc).
				Should(Succeed())
			})
			By("checking the state to be Ready in KCP", func() {
				Eventually(KymaIsInState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
			})
			By("checking the state to be Ready in SKR", func() {
				Eventually(KymaIsInState, Timeout, Interval).
					WithContext(ctx).
					WithArguments(skrKyma.GetName(), skrKyma.GetNamespace(), skrClient, shared.StateReady).
					Should(Succeed())
			})
			By("Kyma status contains expected condition in KCP", func() {
				kymaInCluster, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kymaInCluster.
					ContainsCondition(v1beta2.ConditionTypeModules, apimetav1.ConditionTrue)).
					To(BeTrue())
			})
			By("Kyma status contains expected condition in SKR", func() {
				kymaInCluster, err := GetKyma(ctx, skrClient, skrKyma.GetName(), skrKyma.GetNamespace())
				Expect(err).ShouldNot(HaveOccurred())
				Expect(kymaInCluster.
					ContainsCondition(v1beta2.ConditionTypeModules, apimetav1.ConditionTrue)).
					To(BeTrue())
			})
		})
	}
})

var _ = Describe("Kyma skip Reconciliation", Ordered, func() {
	kyma := NewTestKyma("kyma-test-update")
	module := NewTestModule("skr-module-update", v1beta2.DefaultChannel)
	const moduleVersion = "0.0.1"
	kyma.Spec.Modules = append(
		kyma.Spec.Modules, module)
	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

	It("Should deploy ModuleTemplate", func() {
		data := builder.NewModuleCRBuilder().WithSpec(InitSpecKey, InitSpecValue).Build()
		template := builder.NewModuleTemplateBuilder().
			WithName(module.Name+"-"+moduleVersion).
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(module.Name).
			WithVersion(moduleVersion).
			WithModuleCR(data).
			WithOCM(compdescv2.SchemaVersion).
			WithAnnotation(shared.IsClusterScopedAnnotation, shared.EnableLabelValue).Build()
		Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
	})

	It("Should deploy ModuleReleaseMeta", func() {
		moduleReleaseMeta := ConfigureKCPModuleReleaseMeta(module.Name, module.Channel, moduleVersion)
		Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(moduleReleaseMeta).
			Should(Succeed())

		// descriptor is required to create Manifest
		registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, moduleVersion)
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
			expectManifestSpecDataEquals(kyma.Name, kyma.Namespace, InitSpecValue)),
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

var _ = Describe("Kyma.Spec.Status.Modules.Resource.Namespace should be empty for cluster scoped modules", Ordered,
	func() {
		kyma := NewTestKyma("kyma")
		skrKyma := NewSKRKyma()
		module := NewTestModule("test-module", v1beta2.DefaultChannel)

		const moduleVersion = "0.0.2"
		kyma.Spec.Modules = append(
			kyma.Spec.Modules, module)
		var skrClient client.Client
		var err error
		RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)

		BeforeAll(func() {
			Eventually(func() error {
				skrClient, err = testSkrContextFactory.Get(kyma.GetNamespacedName())
				return err
			}, Timeout, Interval).Should(Succeed())
		})
		BeforeEach(func() {
			By("get latest kyma CR")
			Eventually(SyncKyma, Timeout, Interval).
				WithContext(ctx).WithArguments(skrClient, skrKyma).Should(Succeed())
		})

		It("Should deploy ModuleTemplate", func() {
			for _, module := range kyma.Spec.Modules {
				template := builder.NewModuleTemplateBuilder().
					WithName(module.Name+"-"+moduleVersion).
					WithNamespace(ControlPlaneNamespace).
					WithModuleName(module.Name).
					WithVersion(moduleVersion).
					WithOCM(compdescv2.SchemaVersion).
					WithAnnotation(shared.IsClusterScopedAnnotation, shared.EnableLabelValue).Build()
				Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
					WithArguments(template).
					Should(Succeed())
			}
		})

		It("Should deploy ModuleReleaseMeta", func() {
			moduleReleaseMeta := ConfigureKCPModuleReleaseMeta(module.Name, module.Channel, moduleVersion)
			Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
				WithArguments(moduleReleaseMeta).
				Should(Succeed())

			// descriptor is required to create Manifest
			registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, moduleVersion)
		})

		It("expect Kyma.Spec.Status.Modules.Resource.Namespace to be empty", func() {
			emptyNamespace := ""
			By("ensuring empty Module Status Resource Namespace in KCP")
			Eventually(expectKymaModuleStatusWithNamespace, Timeout, Interval).
				WithContext(ctx).
				WithArguments(kcpClient, kyma, emptyNamespace).
				Should(Succeed())

			By("ensuring empty Module Status Resource Namespace in SKR")
			Eventually(expectKymaModuleStatusWithNamespace, Timeout, Interval).
				WithContext(ctx).
				WithArguments(skrClient, skrKyma, "").
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
		createdKyma, err := GetKyma(ctx, kcpClient, kymaName, ControlPlaneNamespace)
		if err != nil {
			return err
		}
		for _, activeModule := range createdKyma.Spec.Modules {
			return UpdateModuleTemplateSpec(ctx, kcpClient,
				activeModule, InitSpecKey, valueUpdated, createdKyma)
		}
		return nil
	}
}

func expectKymaModuleStatusWithNamespace(ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma,
	expectedNamespace string,
) error {
	createdKyma, err := GetKyma(ctx, clnt, kyma.GetName(), kyma.GetNamespace())
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
}

func PrintModuleTemplates(ctx context.Context, clnt client.Client) {
	fmt.Printf("\n#### ModuleTemplates: ##########################################################\n")
	moduletemplates := v1beta2.ModuleTemplateList{}
	if err := clnt.List(ctx, &moduletemplates); err != nil {
		fmt.Printf("%s", fmt.Errorf("while listing ModuleTemplates: %w", err))
	}
	for idx, mtemplate := range moduletemplates.Items {
		fmt.Printf("ModuleTemplate %d: name: %s, spec.moduleName: %q, spec.version: %q\n",
			idx, mtemplate.Name, mtemplate.Spec.ModuleName, mtemplate.Spec.Version)
	}
	fmt.Printf("################################################################################\n")
}

func PrintModuleReleaseMetas(ctx context.Context, clnt client.Client) {
	fmt.Printf("\n#### ModuleReleaseMetas: #######################################################\n")
	modulereleasemetas := v1beta2.ModuleReleaseMetaList{}
	if err := clnt.List(ctx, &modulereleasemetas); err != nil {
		fmt.Printf("%s", fmt.Errorf("while listing ModuleReleaseMetas: %w", err))
	}
	for idx, mrm := range modulereleasemetas.Items {
		fmt.Printf("ModuleReleaseMeta %d: name: %s, spec.moduleName: %#v, spec.ocmComponentName: %s, channel mapping:",
			idx, mrm.Name, mrm.Spec.ModuleName, mrm.Spec.OcmComponentName)
		for _, mapping := range mrm.Spec.Channels {
			fmt.Printf(" %s->%s;", mapping.Channel, mapping.Version)
		}
		fmt.Println()
	}
	fmt.Printf("################################################################################\n")
}

func PrintManifests(ctx context.Context, clnt client.Client) {
	fmt.Printf("\n#### Manifests: ################################################################\n")
	manifests := v1beta2.ManifestList{}
	if err := clnt.List(ctx, &manifests); err != nil {
		fmt.Printf("%s", fmt.Errorf("while listing Manifests: %w", err))
	}
	for idx, manifest := range manifests.Items {
		fmt.Printf("Manifest %d: name: %s, spec.config: %#v, status.operation: %s\n",
			idx, manifest.Name, manifest.Spec.Config, manifest.Status.Operation)
		ser, _ := json.MarshalIndent(manifest, " ==> ", "  ")
		fmt.Printf("\nManifest %d: %s\n", idx, string(ser))
	}
	fmt.Printf("################################################################################\n")
}

func PrintKymas(ctx context.Context, clnt client.Client) {
	fmt.Printf("\n#### Kymas: ####################################################################\n")
	kymas := v1beta2.KymaList{}
	if err := clnt.List(ctx, &kymas); err != nil {
		fmt.Printf("%s", fmt.Errorf("while listing Kymas: %w", err))
	}
	for idx, kyma := range kymas.Items {
		modules := []string{}
		for _, m := range kyma.Spec.Modules {
			modules = append(modules, m.Name)
		}
		fmt.Printf("Kyma %d: name: %s, spec.modules: %v, status.state: %s, status.operation: %s\n",
			idx, kyma.Name, modules, kyma.Status.State, kyma.Status.Operation)

		ser, _ := json.MarshalIndent(kyma.ObjectMeta, " ==> ", "  ")
		fmt.Printf("\nKyma %d ObjectMeta: %s\n", idx, string(ser))

		ser, _ = json.MarshalIndent(kyma.Spec, " ==> ", "  ")
		fmt.Printf("\nKyma %d spec: %s\n", idx, string(ser))

		ser, _ = json.MarshalIndent(kyma.Status, " ==> ", "  ")
		fmt.Printf("\nKyma %d status: %s\n", idx, string(ser))
	}
	fmt.Printf("################################################################################\n")
}
