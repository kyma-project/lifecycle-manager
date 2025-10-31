package kyma_test

import (
	"context"
	"encoding/json"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	crdv1beta2 "github.com/kyma-project/lifecycle-manager/config/samples/component-integration-installed/crd/v1beta2" //nolint:importas,revive // a one-time reference for the package
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	InitSpecKey   = "initKey"
	InitSpecValue = "initValue"
	ver101        = "1.0.1"
	ver201        = "2.0.1"
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta2.Kyma) {
	const mandatoryModuleName = "mandatory-module"
	const normalModuleVersion = ver101
	const mandatoryModuleVersion = ver201

	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)
	objTracker := &deletionTracker{}
	BeforeAll(func() {
		DeployMandatoryModuleTemplate(ctx, kcpClient, mandatoryModuleName, mandatoryModuleVersion, objTracker)
		DeployModuleTemplates(ctx, kcpClient, kyma, normalModuleVersion, objTracker)
	})

	AfterAll(func() {
		Eventually(objTracker.tryDeleteAll, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient).
			Should(Succeed())
	})
}

func RegisterDefaultLifecycleForKymaWithoutTemplate(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, kyma).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(kcpClient, kyma).Should(Succeed())
	})
}

// DeployModuleTemplates deploys ModuleTemplate and ModuleReleaseMeta for each module in the given Kyma spec.
// It also registers the corresponding OCM descriptors (default empty ones).
// The created resources are tracked by the provided deletionTracker for later cleanup.
func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma,
	version string, tracker *deletionTracker,
) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithName(v1beta2.CreateModuleTemplateName(module.Name, version)).
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(module.Name).
			WithVersion(version).
			Build()
		Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
			WithArguments(kcpClient, template).
			Should(Succeed())
		defer tracker.add(template)
		moduleReleaseMeta := ConfigureKCPModuleReleaseMeta(module.Name, module.Channel, version)
		Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
			WithArguments(kcpClient, moduleReleaseMeta).
			Should(Succeed())
		defer tracker.add(moduleReleaseMeta)
		err := registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, version)
		Expect(err).ShouldNot(HaveOccurred())

		managedModule := NewTestModuleWithFixName(module.Name, module.Channel, "")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, kcpClient, managedModule, kyma).
			Should(Succeed())
	}
}

// DeployMandatoryModuleTemplate deploys a mandatory ModuleTemplate and its corresponding ModuleReleaseMeta.
// It also registers the corresponding OCM descriptor (default empty one).
// The created resources are tracked by the provided deletionTracker for later cleanup.
func DeployMandatoryModuleTemplate(ctx context.Context, kcpClient client.Client, moduleName,
	version string, tracker *deletionTracker,
) {
	mandatoryTemplate := newMandatoryModuleTemplate(moduleName, version)
	Eventually(CreateCR, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, mandatoryTemplate).Should(Succeed())
	defer tracker.add(mandatoryTemplate)
	moduleReleaseMeta := ConfigureKCPMandatoryModuleReleaseMeta(mandatoryTemplate.Spec.ModuleName, version)
	Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
		WithArguments(kcpClient, moduleReleaseMeta).
		Should(Succeed())
	defer tracker.add(moduleReleaseMeta)
	err := registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, version)
	Expect(err).ShouldNot(HaveOccurred())
}

func newMandatoryModuleTemplate(moduleName, version string) *v1beta2.ModuleTemplate {
	return builder.NewModuleTemplateBuilder().
		WithName(v1beta2.CreateModuleTemplateName(moduleName, version)).
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleName).
		WithVersion(version).
		WithMandatory(true).
		Build()
}

func KCPModuleExistWithOverwrites(kyma *v1beta2.Kyma, module v1beta2.Module) string {
	moduleInCluster, err := GetManifest(ctx, kcpClient,
		kyma.GetName(), kyma.GetNamespace(), module.Name)
	Expect(err).ToNot(HaveOccurred())
	manifestSpec := moduleInCluster.Spec
	body, err := json.Marshal(manifestSpec.Resource.Object["spec"])
	Expect(err).ToNot(HaveOccurred())
	kcpModuleSpec := crdv1beta2.KCPModuleSpec{}
	err = json.Unmarshal(body, &kcpModuleSpec)
	Expect(err).ToNot(HaveOccurred())
	return kcpModuleSpec.InitKey
}

func UpdateAllManifestState(kymaName, kymaNamespace string, state shared.State) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, kcpClient, kymaName, kymaNamespace)
		if err != nil {
			return err
		}
		for _, module := range kyma.Spec.Modules {
			if err := UpdateManifestState(ctx, kcpClient,
				kyma.GetName(), kyma.GetNamespace(), module.Name, state); err != nil {
				return err
			}
		}
		return nil
	}
}

func ConfigureKCPModuleReleaseMeta(moduleName, moduleChannel, moduleVersion string) *v1beta2.ModuleReleaseMeta {
	return builder.NewModuleReleaseMetaBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleName).
		WithOcmComponentName(FullOCMName(moduleName)).
		WithSingleModuleChannelAndVersions(moduleChannel, moduleVersion).
		Build()
}

func ConfigureKCPMandatoryModuleReleaseMeta(moduleName, moduleVersion string) *v1beta2.ModuleReleaseMeta {
	return builder.NewModuleReleaseMetaBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleName).
		WithOcmComponentName(FullOCMName(moduleName)).
		WithMandatory(moduleVersion).
		Build()
}

// deletionTracker helps to track created objects and delete them in the end of the test.
// Introduced because manual deletion was very fragile because of big number of independent test cases
// that actually depend on creation and deletion of similar objects.
type deletionTracker struct {
	objects []client.Object
}

func (dt *deletionTracker) add(obj client.Object) {
	dt.objects = append(dt.objects, obj)
}

// tryDeleteAll tries to delete all tracked objects and returns on the first error.
// The remaining objects (including the one for which the deletion failed) are kept for the next try.
func (dt *deletionTracker) tryDeleteAll(ctx context.Context, kcpClient client.Client) error {
	for i, obj := range dt.objects {
		if err := kcpClient.Delete(ctx, obj); err != nil && client.IgnoreNotFound(err) != nil {
			dt.objects = dt.objects[i:] // keep the rest for next try
			return err
		}
		time.Sleep(50 * time.Millisecond) // slight delay to avoid overwhelming the API server
	}
	return nil
}
