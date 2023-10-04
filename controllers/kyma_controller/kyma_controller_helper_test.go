package kyma_controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	crdV1beta2 "github.com/kyma-project/lifecycle-manager/config/samples/component-integration-installed/crd/v1beta2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta2.Kyma) {
	BeforeAll(func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma, false, false, false,
			false)
	})

	AfterAll(func() {
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma, false)
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

func GetKymaState(kymaName string) (string, error) {
	createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return "", err
	}
	return string(createdKyma.Status.State), nil
}

func GetKymaModulesStatus(kymaName string) []v1beta2.ModuleStatus {
	createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return []v1beta2.ModuleStatus{}
	}
	return createdKyma.Status.Modules
}

func GetKymaConditions(kymaName string) []metav1.Condition {
	createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return []metav1.Condition{}
	}
	return createdKyma.Status.Conditions
}

func UpdateKymaLabel(
	ctx context.Context,
	client client.Client,
	kyma *v1beta2.Kyma,
	labelKey,
	labelValue string,
) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, client, kyma.Name, kyma.Namespace)
		if err != nil {
			return err
		}
		kyma.Labels[labelKey] = labelValue
		return client.Update(ctx, kyma)
	}
}

func KCPModuleExistWithOverwrites(kyma *v1beta2.Kyma, module v1beta2.Module) string {
	kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
	Expect(err).ToNot(HaveOccurred())
	moduleInCluster, err := GetManifest(ctx, controlPlaneClient, kyma, module)
	Expect(err).ToNot(HaveOccurred())
	manifestSpec := moduleInCluster.Spec
	body, err := json.Marshal(manifestSpec.Resource.Object["spec"])
	Expect(err).ToNot(HaveOccurred())
	kcpModuleSpec := crdV1beta2.KCPModuleSpec{}
	err = json.Unmarshal(body, &kcpModuleSpec)
	Expect(err).ToNot(HaveOccurred())
	return kcpModuleSpec.InitKey
}

func deleteModule(kyma *v1beta2.Kyma, module v1beta2.Module) func() error {
	return func() error {
		component, err := GetManifest(ctx, controlPlaneClient, kyma, module)
		if util.IsNotFound(err) {
			return nil
		}
		return client.IgnoreNotFound(controlPlaneClient.Delete(ctx, component))
	}
}

func UpdateKymaModuleChannels(kymaName, channel string) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return err
	}
	for i := range kyma.Spec.Modules {
		kyma.Spec.Modules[i].Channel = channel
	}
	return controlPlaneClient.Update(ctx, kyma)
}

var ErrTemplateInfoChannelMismatch = errors.New("mismatch in template info channel")

func TemplateInfosMatchChannel(kymaName, channel string) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return err
	}
	for i := range kyma.Status.Modules {
		if kyma.Status.Modules[i].Channel != channel {
			return fmt.Errorf(
				"%w: %s should be %s",
				ErrTemplateInfoChannelMismatch, kyma.Status.Modules[i].Channel, channel,
			)
		}
	}
	return nil
}

func CreateModuleTemplateSetsForKyma(modules []v1beta2.Module, modifiedVersion, channel string) error {
	for _, module := range modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, false, false, false, false)
		if err != nil {
			return err
		}

		descriptor, err := template.GetDescriptor(false)
		if err != nil {
			return err
		}
		descriptor.Version = modifiedVersion
		newDescriptor, err := compdesc.Encode(descriptor.ComponentDescriptor, compdesc.DefaultJSONLCodec)
		if err != nil {
			return err
		}
		template.Spec.Descriptor.Raw = newDescriptor
		template.Spec.Channel = channel
		template.Name = fmt.Sprintf("%s-%s", template.Name, channel)
		if err := controlPlaneClient.Create(ctx, template); err != nil {
			return err
		}
	}
	return nil
}

func UpdateAllManifestState(kymaName string, state v1beta2.State) func() error {
	return func() error {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return err
		}
		for _, module := range createdKyma.Spec.Modules {
			if err := UpdateManifestState(ctx, controlPlaneClient, createdKyma, module, state); err != nil {
				return err
			}
		}
		return nil
	}
}
