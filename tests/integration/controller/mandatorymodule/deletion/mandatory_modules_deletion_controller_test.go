package mandatory_test

import (
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

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

		It("Then Kyma CR should result in a ready state immediately as there are no modules", func() {
			Eventually(KymaIsInState).
				WithContext(ctx).
				WithArguments(kyma.GetName(), kyma.GetNamespace(), controlPlaneClient, shared.StateReady).
				Should(Succeed())
		})
	})
})

func registerControlPlaneLifecycleForKyma(kyma *v1beta2.Kyma) {
	template := builder.NewModuleTemplateBuilder().
		WithModuleName("mandatory-module").
		WithChannel(mandatoryChannel).
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()

	descriptor, err := template.GetDescriptor()
	Eventually(err).Should(Succeed())

	mandatoryManifest := &v1beta2.Manifest{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:        "mandatory-module",
			Namespace:   apimetav1.NamespaceDefault,
			Annotations: map[string]string{shared.FQDN: descriptor.GetName()},
		},
		Spec: v1beta2.ManifestSpec{
			Remote: false,
			Install: v1beta2.InstallInfo{
				Source: machineryruntime.RawExtension{
					Raw: []byte{'d'},
				},
				Name: v1beta2.RawManifestLayerName,
			},
		},
		Status: shared.Status{State: shared.StateReady},
	}

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
		Eventually(CreateCR).
			WithContext(ctx).
			WithArguments(controlPlaneClient, mandatoryManifest).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
		Eventually(DeleteCR).
			WithContext(ctx).
			WithArguments(controlPlaneClient, template).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
}
