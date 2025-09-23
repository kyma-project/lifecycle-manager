package mandatory_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
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
	mandatoryChannel = "dummychannel"
	mandatoryModule  = "mandatory-module"
)

var _ = Describe("Mandatory Module Deletion", Ordered, func() {
	Context("Given Kyma with one mandatory Module Manifest CR on Control-Plane", func() {
		kyma := NewTestKyma("no-module-kyma")
		registerControlPlaneLifecycleForKyma(kyma)
		It("Then Kyma CR should result in a ready state and mandatory manifest is created with IsMandatory label",
			func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
					Should(Succeed())
				Eventually(MandatoryManifestExistsWithLabelAndAnnotation).
					WithContext(ctx).
					WithArguments(kcpClient, shared.FQDN, DefaultFQDN).
					Should(Succeed())
				By("And mandatory finalizer is added to the mandatory ModuleTemplate", func() {
					Eventually(mandatoryModuleTemplateFinalizerExists).
						WithContext(ctx).
						WithArguments(kcpClient, client.ObjectKey{
							Namespace: ControlPlaneNamespace,
							Name:      mandatoryModule,
						}).
						Should(Succeed())
				})
				By("And mandatory Module label is add ")
			})

		It("When mandatory ModuleTemplate marked for deletion", func() {
			Eventually(deleteMandatoryModuleTemplates).
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
						Name:      mandatoryModule,
					}).
					Should(Not(Succeed()))
			})
		})
	})
})

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	template := builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithName(mandatoryModule).
		WithChannel(mandatoryChannel).
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).
		WithLabel(shared.IsMandatoryModule, shared.EnableLabelValue).Build()
	mandatoryManifest := NewTestManifest("mandatory-module")
	mandatoryManifest.Spec.Version = "1.1.1-e2e-test"
	mandatoryManifest.Labels[shared.IsMandatoryModule] = "true"

	BeforeAll(func() {
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
		// Set labels and state manual, since we do not start the Kyma Controller
		kyma.Labels[shared.ManagedBy] = shared.OperatorName
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(SetKymaState).
			WithContext(ctx).
			WithArguments(kyma, reconciler, shared.StateReady).Should(Succeed())

		installName := filepath.Join("main-dir", "installs")
		mandatoryManifest.Annotations = map[string]string{shared.FQDN: DefaultFQDN}
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

func deleteMandatoryModuleTemplates(ctx context.Context, clnt client.Client) error {
	templates := v1beta2.ModuleTemplateList{}
	if err := clnt.List(ctx, &templates); err != nil {
		return fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	for _, template := range templates.Items {
		if template.Spec.Mandatory {
			if err := clnt.Delete(ctx, &template); err != nil {
				return fmt.Errorf("failed to delete ModuleTemplate: %w", err)
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
