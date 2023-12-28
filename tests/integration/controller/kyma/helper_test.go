package kyma_test

import (
	"context"
	"encoding/json"

	compdescv2 "github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc/versions/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	crdv1beta2 "github.com/kyma-project/lifecycle-manager/config/samples/component-integration-installed/crd/v1beta2" //nolint:importas // a one-time reference for the package
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	InitSpecKey   = "initKey"
	InitSpecValue = "initValue"
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta2.Kyma) {
	mandatoryTemplate := builder.NewModuleTemplateBuilder().
		WithModuleName("mandatory-template-operator").
		WithChannel(v1beta2.DefaultChannel).
		WithMandatory(true).
		WithOCM(compdescv2.SchemaVersion).Build()

	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, mandatoryTemplate).Should(Succeed())
		DeployModuleTemplates(ctx, controlPlaneClient, kyma)
	})

	AfterAll(func() {
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma)
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, mandatoryTemplate).Should(Succeed())
	})
	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)
}

func RegisterDefaultLifecycleForKymaWithoutTemplate(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		Eventually(CreateCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(DeleteCR, Timeout, Interval).
			WithContext(ctx).
			WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).
			WithContext(ctx).WithArguments(controlPlaneClient, kyma).Should(Succeed())
	})
}

func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1beta2.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template := builder.NewModuleTemplateBuilder().
			WithModuleName(module.Name).
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
			WithModuleName(module.Name).
			WithChannel(module.Channel).
			WithOCM(compdescv2.SchemaVersion).Build()
		Eventually(kcpClient.Create, Timeout, Interval).WithContext(ctx).
			WithArguments(template).
			Should(Succeed())
	}
}

func KCPModuleExistWithOverwrites(kyma *v1beta2.Kyma, module v1beta2.Module) string {
	moduleInCluster, err := GetManifest(ctx, controlPlaneClient,
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
		kyma, err := GetKyma(ctx, controlPlaneClient, kymaName, kymaNamespace)
		if err != nil {
			return err
		}
		for _, module := range kyma.Spec.Modules {
			if err := UpdateManifestState(ctx, controlPlaneClient,
				kyma.GetName(), kyma.GetNamespace(), module.Name, state); err != nil {
				return err
			}
		}
		return nil
	}
}
