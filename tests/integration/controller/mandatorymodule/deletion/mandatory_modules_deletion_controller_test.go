package mandatory_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var ErrNoMandatoryManifest = errors.New("manifest for mandatory Module not found")

const (
	mandatoryChannel = "dummychannel"
)

var _ = Describe("Mandatory Module Deletion", Ordered, func() {
	Context("Given Kyma with one mandatory Module Manifest CR on Control-Plane", func() {
		kyma := NewTestKyma("no-module-kyma")
		registerControlPlaneLifecycleForKyma(kyma)
		It("Then Kyma CR should result in a ready state and mandatory manifest is created with IsMandatory label",
			func() {
				Eventually(KymaIsInState).
					WithContext(ctx).
					WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
					Should(Succeed())
				Eventually(MandatoryManifestExistsWithLabelAndAnnotation).
					WithContext(ctx).
					WithArguments(controlPlaneClient, shared.FQDN, "kyma-project.io/template-operator").
					Should(Succeed())
				By("And mandatory finalizer is added to the mandatory ModuleTemplate", func() {
					Eventually(mandatoryModuleTemplateFinalizerExists).
						WithContext(ctx).
						WithArguments(controlPlaneClient, client.ObjectKey{
							Namespace: apimetav1.NamespaceDefault,
							Name:      "mandatory-module",
						}).
						Should(Succeed())
				})
				By("And mandatory Module label is add ")
			})

		It("When mandatory ModuleTemplate marked for deletion", func() {
			Eventually(deleteMandatoryModuleTemplates).
				WithContext(ctx).
				WithArguments(controlPlaneClient).
				Should(Succeed())
		})
		It("Then mandatory Manifest is deleted", func() {
			Eventually(MandatoryManifestExistsWithLabelAndAnnotation).
				WithContext(ctx).
				WithArguments(controlPlaneClient, shared.FQDN, "kyma-project.io/template-operator").
				Should(Not(Succeed()))
			By("And finalizer is removed from mandatory ModuleTemplate", func() {
				Eventually(mandatoryModuleTemplateFinalizerExists).
					WithContext(ctx).
					WithArguments(controlPlaneClient, client.ObjectKey{
						Namespace: apimetav1.NamespaceDefault,
						Name:      "mandatory-module",
					}).
					Should(Not(Succeed()))
			})
		})
	})
})

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	template := builder.NewModuleTemplateBuilder().
		WithName("mandatory-module").
		WithModuleName("mandatory-module").
		WithChannel(mandatoryChannel).
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()
	mandatoryManifest := NewTestManifest("mandatory-module")
	mandatoryManifest.Labels[shared.IsMandatoryModule] = "true"

	BeforeAll(func() {
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(controlPlaneClient, template).Should(Succeed())
		// Set labels and state manual, since we do not start the Kyma Controller
		kyma.Labels[shared.ManagedBy] = shared.OperatorName
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		Eventually(SetKymaState).
			WithContext(ctx).
			WithArguments(kyma, mandatoryModuleDeletionReconciler, shared.StateReady).Should(Succeed())

		installName := filepath.Join("main-dir", "installs")
		mandatoryManifest.Annotations = map[string]string{shared.FQDN: "kyma-project.io/template-operator"}
		validImageSpec, err := CreateOCIImageSpec(installName, server.Listener.Addr().String(), manifestFilePath,
			false)
		Expect(err).NotTo(HaveOccurred())
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).NotTo(HaveOccurred())
		Eventually(InstallManifest).
			WithContext(ctx).
			WithArguments(controlPlaneClient, mandatoryManifest, imageSpecByte, false).
			Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
}

func deleteMandatoryModuleTemplates(ctx context.Context, clnt client.Client) error {
	templates := v1beta2.ModuleTemplateList{}
	if err := clnt.List(ctx, &templates); err != nil {
		return fmt.Errorf("failed to list ModuleTemplates: %w", err)
	}

	for _, template := range templates.Items {
		template := template
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
	return fmt.Errorf("ModuleTemplate does not contain mandatory finalizer")
}
