package mandatory_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	mandatoryModuleName    = "mandatory-module"
	mandatoryModuleVersion = "1.0.1"
)

var _ = Describe("Mandatory Module Deletion", Ordered, func() {
	Context("Given Kyma with one mandatory Module Manifest CR on Control-Plane", func() {
		kyma := NewTestKyma("no-module-kyma")
		registerControlPlaneLifecycleForKyma(kyma, mandatoryModuleName)
		It("Then Kyma CR should result in a ready state and mandatory manifest is created with IsMandatory label",
			func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
				Eventually(MandatoryManifestExistsWithLabelAndAnnotation).
					WithContext(ctx).
					WithArguments(kcpClient, shared.FQDN, FullOCMName(mandatoryModuleName)).
					Should(Succeed())
				By("And mandatory finalizer is added to the mandatory ModuleTemplate", func() {
					Eventually(mandatoryModuleTemplateFinalizerExists).
						WithContext(ctx).
						WithArguments(kcpClient, client.ObjectKey{
							Namespace: ControlPlaneNamespace,
							Name:      v1beta2.CreateModuleTemplateName(mandatoryModuleName, mandatoryModuleVersion),
						}).
						Should(Succeed())
				})
				By("And mandatory Module label is add ")
			})

		It("When mandatory ModuleTemplate marked for deletion", func() {
			Eventually(deleteMandatoryModule).
				WithContext(ctx).
				WithArguments(kcpClient).
				Should(Succeed())
		})
		It("Then mandatory Manifest is deleted", func() {
			Eventually(MandatoryManifestExistsWithLabelAndAnnotation).
				WithContext(ctx).
				WithArguments(kcpClient, shared.FQDN, DefaultFQDN).
				Should(Not(Succeed()))
			By("And finalizer is removed from mandatory ModuleTemplate", func() {
				Eventually(mandatoryModuleTemplateFinalizerExists).
					WithContext(ctx).
					WithArguments(kcpClient, client.ObjectKey{
						Namespace: ControlPlaneNamespace,
						Name:      mandatoryModuleName,
					}).
					Should(Not(Succeed()))
			})
		})
	})
})

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma, mandatoryModuleName string) {
	template := builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithName(v1beta2.CreateModuleTemplateName(mandatoryModuleName, mandatoryModuleVersion)).
		WithModuleName(mandatoryModuleName).
		WithLabel(shared.IsMandatoryModule, shared.EnableLabelValue).
		WithVersion(mandatoryModuleVersion).
		WithMandatory(true).
		Build()
	moduleReleaseMeta := ConfigureKCPMandatoryModuleReleaseMeta(template.Spec.ModuleName, template.Spec.Version)

	mandatoryManifest := NewTestManifest("mandatory-module")
	mandatoryManifest.Labels[shared.IsMandatoryModule] = "true"
	mandatoryManifest.Annotations = map[string]string{shared.FQDN: moduleReleaseMeta.Spec.OcmComponentName}
	mandatoryManifest.Spec.Version = mandatoryModuleVersion

	BeforeAll(func() {
		err := registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, template.Spec.Version)
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(kcpClient, moduleReleaseMeta).Should(Succeed())
		// Set labels and state manual, since we do not start the Kyma Controller
		kyma.Labels[shared.ManagedBy] = shared.OperatorName
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(SetKymaState).
			WithContext(ctx).
			WithArguments(kyma, reconciler, shared.StateReady).Should(Succeed())

		installName := filepath.Join("main-dir", "installs")
		validImageSpec, err := CreateOCIImageSpecFromFile(installName, server.Listener.Addr().String(),
			manifestFilePath)
		Expect(err).NotTo(HaveOccurred())
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).NotTo(HaveOccurred())
		Eventually(InstallManifest).
			WithContext(ctx).
			WithArguments(kcpClient, mandatoryManifest, imageSpecByte, false).
			Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	})
}

func deleteMandatoryModule(ctx context.Context, clnt client.Client) error {
	templates := v1beta2.ModuleTemplateList{}
	if err := clnt.List(ctx, &templates); err != nil {
		return fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	for _, template := range templates.Items {
		if template.Spec.Mandatory {
			if err := clnt.Delete(ctx, &template); err != nil {
				return fmt.Errorf("failed to delete ModuleTemplate: %w", err)
			}
			moduleReleaseMeta := v1beta2.ModuleReleaseMeta{}
			err := clnt.Get(ctx, client.ObjectKey{
				Namespace: template.Namespace,
				Name:      template.Spec.ModuleName,
			}, &moduleReleaseMeta)
			if err != nil && !errors.Is(err, client.IgnoreNotFound(err)) {
				return fmt.Errorf("failed to get ModuleReleaseMeta: %w", err)
			}
			if err := clnt.Delete(ctx, &moduleReleaseMeta); err != nil {
				return fmt.Errorf("failed to delete ModuleReleaseMeta: %w", err)
			}
		}
	}

	return nil
}

func mandatoryModuleTemplateFinalizerExists(ctx context.Context, clnt client.Client, obj client.ObjectKey) error {
	template := v1beta2.ModuleTemplate{}
	if err := clnt.Get(ctx, obj, &template); err != nil {
		return fmt.Errorf("failed to get ModuleTemplate: %w", err)
	}

	if controllerutil.ContainsFinalizer(&template, "operator.kyma-project.io/mandatory-module") {
		return nil
	}
	return errors.New("ModuleTemplate does not contain mandatory finalizer")
}

func ConfigureKCPMandatoryModuleReleaseMeta(moduleName, moduleVersion string) *v1beta2.ModuleReleaseMeta {
	return builder.NewModuleReleaseMetaBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleName).
		WithOcmComponentName(FullOCMName(moduleName)).
		WithMandatory(moduleVersion).
		Build()
}
