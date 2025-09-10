package kyma_test

import (
	"context"
	"encoding/json"
	"fmt"

	compdescv2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
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
	InitSpecKey      = "initKey"
	InitSpecValue    = "initValue"
	mandatoryChannel = "dummychannel"
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta2.Kyma) {
	const normalModuleVersion = "1.0.1"
	const mandatoryModuleVersion = "2.0.1"
	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)
	BeforeAll(func() {
		DeployMandatoryModuleTemplate(ctx, kcpClient, mandatoryModuleVersion)
		DeployModuleTemplates(ctx, kcpClient, kyma, normalModuleVersion)
	})

	AfterAll(func() {
		DeleteModuleTemplates(ctx, kcpClient, kyma, normalModuleVersion)
		DeleteMandatoryModuleTemplate(ctx, kcpClient, mandatoryModuleVersion)
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

func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma, version string) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithName(createModuleTemplateName(module, version)).
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCM(compdescv2.SchemaVersion).Build()
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
		moduleReleaseMeta := ConfigureKCPModuleReleaseMeta(module.Name, module.Channel, version)
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, moduleReleaseMeta).Should(Succeed())
	}
}

func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma, version string) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithName(createModuleTemplateName(module, version)).
			WithNamespace(ControlPlaneNamespace).
			WithModuleName(module.Name).
			WithVersion(version).
			WithOCM(compdescv2.SchemaVersion).Build()
		Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
			WithArguments(kcpClient, template).
			Should(Succeed())
		moduleReleaseMeta := ConfigureKCPModuleReleaseMeta(module.Name, module.Channel, version)
		Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
			WithArguments(kcpClient, moduleReleaseMeta).
			Should(Succeed())
		registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, version)

		managedModule := NewTestModuleWithFixName(module.Name, module.Channel, "")

		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, kcpClient, managedModule, kyma).
			Should(Succeed())
	}
}

func DeployMandatoryModuleTemplate(ctx context.Context, kcpClient client.Client, version string) {
	mandatoryTemplate := newMandatoryModuleTemplate(version)
	Eventually(CreateCR, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, mandatoryTemplate).Should(Succeed())
	moduleReleaseMeta := ConfigureKCPMandatoryModuleReleaseMeta(mandatoryTemplate.Spec.ModuleName, version)
	Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
		WithArguments(kcpClient, moduleReleaseMeta).
		Should(Succeed())
	registerDescriptor(moduleReleaseMeta.Spec.OcmComponentName, version)
}

func DeleteMandatoryModuleTemplate(ctx context.Context, kcpClient client.Client, version string) {
	mandatoryTemplate := newMandatoryModuleTemplate(version)
	Eventually(DeleteCR, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, mandatoryTemplate).Should(Succeed())
}

func createModuleTemplateName(module v1beta2.Module, version string) string {
	return fmt.Sprintf("%s-%s", module.Name, version)
}

func newMandatoryModuleTemplate(version string) *v1beta2.ModuleTemplate {
	return builder.NewModuleTemplateBuilder().
		WithName("mandatory-template-operator" + "-" + version).
		WithNamespace(ControlPlaneNamespace).
		WithModuleName("mandatory-template-operator").
		WithVersion(version).
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()
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
		WithOcmComponentName(v1beta2.FullOCMName(moduleName)).
		WithSingleModuleChannelAndVersions(moduleChannel, moduleVersion).
		Build()
}

func ConfigureKCPMandatoryModuleReleaseMeta(moduleName, moduleVersion string) *v1beta2.ModuleReleaseMeta {
	return builder.NewModuleReleaseMetaBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithModuleName(moduleName).
		WithOcmComponentName(v1beta2.FullOCMName(moduleName)).
		WithMandatory(moduleVersion).
		Build()
}
