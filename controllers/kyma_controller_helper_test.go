package controllers_test

import (
	"encoding/json"
	"errors"
	"fmt"

	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	sampleCRDv1alpha1 "github.com/kyma-project/lifecycle-manager/config/samples/component-integration-installed/crd/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/watch"
	manifestV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
)

func RegisterDefaultLifecycleForKyma(kyma *v1alpha1.Kyma) {
	BeforeAll(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
		DeployModuleTemplates(ctx, controlPlaneClient, kyma)
	})

	AfterAll(func() {
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma)
	})

	AfterAll(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Expect(controlPlaneClient.Get(ctx, client.ObjectKey{
			Name:      kyma.Name,
			Namespace: metav1.NamespaceDefault,
		}, kyma)).Should(Succeed())
	})
}

func GetKymaState(kymaName string) func() string {
	return func() string {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName)
		if err != nil {
			return ""
		}
		return string(createdKyma.Status.State)
	}
}

func GetKymaConditions(kymaName string) func() []metav1.Condition {
	return func() []metav1.Condition {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName)
		if err != nil {
			return []metav1.Condition{}
		}
		return createdKyma.Status.Conditions
	}
}

func UpdateModuleState(kymaName, moduleName string, state v1alpha1.State) func() error {
	return func() error {
		return updateModuleState(kymaName, moduleName, state)
	}
}

func updateModuleState(kymaName string, moduleName string, state v1alpha1.State) error {
	component, err := getModule(kymaName, moduleName)
	Expect(err).ShouldNot(HaveOccurred())
	component.Object[watch.Status] = map[string]any{watch.State: string(state)}
	return k8sManager.GetClient().Status().Update(ctx, component)
}

func ModuleExists(kymaName, moduleName string) func() bool {
	return func() bool {
		_, err := getModule(kymaName, moduleName)
		return err == nil
	}
}

func ModuleNotExist(kymaName string, moduleName string) func() bool {
	return func() bool {
		_, err := getModule(kymaName, moduleName)
		return k8serrors.IsNotFound(err)
	}
}

func SKRModuleExistWithOverwrites(kymaName string, moduleName string) func() string {
	return func() string {
		module, err := getModule(kymaName, moduleName)
		Expect(err).ToNot(HaveOccurred())
		manifestSpec := UnmarshalManifestSpec(module)
		body, err := json.Marshal(manifestSpec.Resource.Object["spec"])
		Expect(err).ToNot(HaveOccurred())
		skrModuleSpec := sampleCRDv1alpha1.SKRModuleSpec{}
		err = json.Unmarshal(body, &skrModuleSpec)
		Expect(err).ToNot(HaveOccurred())
		return skrModuleSpec.InitKey
	}
}

func UnmarshalManifestSpec(module *unstructured.Unstructured) *manifestV1alpha1.ManifestSpec {
	body, err := json.Marshal(module.Object["spec"])
	Expect(err).ToNot(HaveOccurred())
	manifestSpec := manifestV1alpha1.ManifestSpec{}
	err = json.Unmarshal(body, &manifestSpec)
	Expect(err).ToNot(HaveOccurred())
	return &manifestSpec
}

func getModule(kymaName, moduleName string) (*unstructured.Unstructured, error) {
	component := &unstructured.Unstructured{}
	component.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.OperatorPrefix,
		Version: v1alpha1.Version,
		Kind:    "Manifest",
	})
	err := controlPlaneClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      common.CreateModuleName(moduleName, kymaName),
	}, component)
	if err != nil {
		return nil, err
	}
	return component, nil
}

func GetModuleTemplate(name string) (*v1alpha1.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1alpha1.ModuleTemplate{}
	err := controlPlaneClient.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: name},
		moduleTemplateInCluster)
	if err != nil {
		return nil, err
	}
	return moduleTemplateInCluster, nil
}

func RemoteKymaExists(remoteClient client.Client, kymaName string) func() error {
	return func() error {
		_, err := GetKyma(ctx, remoteClient, kymaName)
		return err
	}
}

func ModuleTemplatesExist(clnt client.Client, kyma *v1alpha1.Kyma) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			if err != nil {
				return err
			}
			if err := clnt.Get(ctx, client.ObjectKeyFromObject(template), template); err != nil {
				return err
			}
		}

		return nil
	}
}

var ErrModuleTemplateDescriptorLabelCountMismatch = errors.New("label count in descriptor does not match")

func ModuleTemplatesLabelsCountMatch(
	clnt client.Client, kyma *v1alpha1.Kyma, count int,
) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			if err != nil {
				return err
			}
			if err := clnt.Get(ctx, client.ObjectKeyFromObject(template), template); err != nil {
				return err
			}

			descriptor, err := template.Spec.GetUnsafeDescriptor()
			if err != nil {
				return err
			}

			if len(descriptor.GetLabels()) != count {
				return fmt.Errorf("expected %v but got %v labels: %w", count,
					len(descriptor.GetLabels()), ErrModuleTemplateDescriptorLabelCountMismatch,
				)
			}
		}
		return nil
	}
}

func ModifyModuleTemplateSpecThroughLabels(clnt client.Client, kyma *v1alpha1.Kyma, labels []ocm.Label) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			if err != nil {
				return err
			}

			if err := clnt.Get(ctx, client.ObjectKeyFromObject(template), template); err != nil {
				return err
			}

			err = template.Spec.ModifyDescriptor(
				func(descriptor *ocm.ComponentDescriptor) error {
					descriptor.SetLabels(labels)
					return nil
				})
			if err != nil {
				return err
			}

			if err := runtimeClient.Update(ctx, template); err != nil {
				return err
			}
		}

		return nil
	}
}

func deleteModule(kymaName, moduleName string) func() error {
	return func() error {
		component := &unstructured.Unstructured{}
		component.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   v1alpha1.OperatorPrefix,
			Version: v1alpha1.Version,
			Kind:    "Manifest",
		})
		component.SetNamespace(metav1.NamespaceDefault)
		component.SetName(common.CreateModuleName(moduleName, kymaName))
		err := controlPlaneClient.Delete(ctx, component)
		return client.IgnoreNotFound(err)
	}
}

func UpdateKymaModuleChannels(kymaName string, channel v1alpha1.Channel) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName)
	if err != nil {
		return err
	}
	for i := range kyma.Spec.Modules {
		kyma.Spec.Modules[i].Channel = channel
	}
	if err := controlPlaneClient.Update(ctx, kyma); err != nil {
		return err
	}
	return nil
}

var ErrTemplateInfoChannelMismatch = errors.New("mismatch in template info channel")

func TemplateInfosMatchChannel(kymaName string, channel v1alpha1.Channel) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName)
	if err != nil {
		return err
	}
	for i := range kyma.Status.ModuleStatus {
		if kyma.Status.ModuleStatus[i].TemplateInfo.Channel != channel {
			return fmt.Errorf("%w: %s should be %s",
				ErrTemplateInfoChannelMismatch, kyma.Status.ModuleStatus[i].TemplateInfo.Channel, channel)
		}
	}
	return nil
}
