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

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	InitSpecKey      = "initKey"
	InitSpecValue    = "initValue"
	mandatoryChannel = "dummychannel"
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta2.Kyma) {
	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)
	BeforeAll(func() {
		DeployMandatoryModuleTemplate(ctx, kcpClient)
		DeployModuleTemplates(ctx, kcpClient, kyma)
	})

	AfterAll(func() {
		DeleteModuleTemplates(ctx, kcpClient, kyma)
		DeleteMandatoryModuleTemplate(ctx, kcpClient)
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

func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithNamespace(ControlPlaneNamespace).
			WithName(createModuleTemplateName(module)).
			WithLabelModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCM(compdescv2.SchemaVersion).Build()
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kcpClient, template).Should(Succeed())
	}
}

func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithName(createModuleTemplateName(module)).
			WithNamespace(ControlPlaneNamespace).
			WithLabelModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCM(compdescv2.SchemaVersion).Build()
		Eventually(CreateCR, Timeout, Interval).WithContext(ctx).
			WithArguments(kcpClient, template).
			Should(Succeed())
		managedModule := NewTestModuleWithFixName(module.Name, module.Channel, "")
		Eventually(ModuleTemplateExists, Timeout, Interval).
			WithArguments(ctx, kcpClient, managedModule, kyma).
			Should(Succeed())
	}
}

func DeployMandatoryModuleTemplate(ctx context.Context, kcpClient client.Client) {
	mandatoryTemplate := newMandatoryModuleTemplate()
	Eventually(CreateCR, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, mandatoryTemplate).Should(Succeed())
}

func DeleteMandatoryModuleTemplate(ctx context.Context, kcpClient client.Client) {
	mandatoryTemplate := newMandatoryModuleTemplate()
	Eventually(DeleteCR, Timeout, Interval).
		WithContext(ctx).
		WithArguments(kcpClient, mandatoryTemplate).Should(Succeed())
}

func createModuleTemplateName(module v1beta2.Module) string {
	return fmt.Sprintf("%s-%s", module.Name, module.Channel)
}

func newMandatoryModuleTemplate() *v1beta2.ModuleTemplate {
	return builder.NewModuleTemplateBuilder().
		WithNamespace(ControlPlaneNamespace).
		WithName("mandatory-template").
		WithLabelModuleName("mandatory-template-operator").
		WithChannel(mandatoryChannel).
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
