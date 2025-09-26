package mandatory_test

import (
	"context"
	"errors"

	k8slabels "k8s.io/apimachinery/pkg/labels"
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
	ErrWrongModulesStatus  = errors.New("modules status not correct")
	ErrNoMandatoryManifest = errors.New("manifest for mandatory Module not found")
)

const (
	mandatoryChannel = "dummychannel"
)

var _ = Describe("Mandatory Module Installation", Ordered, func() {
	Context("Given Kyma with no Module and one mandatory ModuleTemplate on Control-Plane", func() {
		kyma := NewTestKyma("no-module-kyma")
		registerControlPlaneLifecycleForKyma(kyma)

		It("Then Kyma CR should result in a ready state immediately as there are no modules", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), kcpClient, shared.StateReady).
				Should(Succeed())
		})

		It("And Kyma CR should contain empty status.modules", func() {
			Eventually(func() error {
				createdKyma, err := GetKyma(ctx, kcpClient, kyma.GetName(), kyma.GetNamespace())
				if err != nil {
					return err
				}
				if len(createdKyma.Status.Modules) != 0 {
					return ErrWrongModulesStatus
				}
				return nil
			}).
				Should(Succeed())
		})

		It("And Manifest CR for the Mandatory Module should be created with correct Owner Reference", func() {
			Eventually(checkMandatoryManifestForKyma).
				WithContext(ctx).
				WithArguments(kyma, DefaultFQDN).
				Should(Succeed())
		})
	})
})

var _ = Describe("Skipping Mandatory Module Installation", Ordered, func() {
	Context("Given Kyma with no Module and one mandatory ModuleTemplate on Control-Plane", func() {
		kyma := NewTestKyma("skip-reconciliation-kyma")
		kyma.Labels[shared.SkipReconcileLabel] = "true"
		registerControlPlaneLifecycleForKyma(kyma)

		It("When Kyma has 'skip-reconciliation' label, then no Mandatory Module Manifest should be created", func() {
			Eventually(checkMandatoryManifestForKyma).
				WithContext(ctx).
				WithArguments(kyma, DefaultFQDN).
				Should(Equal(ErrNoMandatoryManifest))
		})
	})
})

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	template := builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName("mandatory-module").
		WithLabel(shared.IsMandatoryModule, shared.EnableLabelValue).
		WithChannel(mandatoryChannel).
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()

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
	})

	AfterAll(func() {
		Eventually(DeleteCR).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
		Eventually(DeleteCR).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	})
}

func checkMandatoryManifestForKyma(ctx context.Context, kyma *v1beta2.Kyma, fqdn string) error {
	manifestList := v1beta2.ManifestList{}
	if err := reconciler.List(ctx, &manifestList, &client.ListOptions{
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.KymaName: kyma.Name}),
	}); err != nil {
		return err
	}
	for _, manifest := range manifestList.Items {
		if manifest.OwnerReferences[0].Name == kyma.Name &&
			manifest.Annotations[shared.FQDN] == fqdn {
			return nil
		}
	}
	return ErrNoMandatoryManifest
}
