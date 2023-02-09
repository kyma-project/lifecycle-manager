package controllers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	declarative "github.com/kyma-project/lifecycle-manager/pkg/declarative/v2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"

	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Masterminds/semver/v3"
	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	sampleCRDv1alpha1 "github.com/kyma-project/lifecycle-manager/config/samples/component-integration-installed/crd/v1alpha1"
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
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return ""
		}
		return string(createdKyma.Status.State)
	}
}

func GetKymaConditions(kymaName string) func() []metav1.Condition {
	return func() []metav1.Condition {
		createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
		if err != nil {
			return []metav1.Condition{}
		}
		return createdKyma.Status.Conditions
	}
}

func UpdateModuleState(
	ctx context.Context, kyma *v1alpha1.Kyma, module v1alpha1.Module, state v1alpha1.State,
) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		if err != nil {
			return err
		}
		return updateModuleState(kyma, module, state)
	}
}

func updateModuleState(kyma *v1alpha1.Kyma, module v1alpha1.Module, state v1alpha1.State) error {
	component, err := getModule(kyma, module)
	if err != nil {
		return err
	}
	component.Status.State = declarative.State(state)
	return k8sManager.GetClient().Status().Update(ctx, component)
}

func ModuleExists(ctx context.Context, kyma *v1alpha1.Kyma, module v1alpha1.Module) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, controlPlaneClient, kyma.Name, kyma.Namespace)
		if err != nil {
			return err
		}
		_, err = getModule(kyma, module)
		return err
	}
}

func UpdateRemoteModule(ctx context.Context,
	client client.Client,
	kyma *v1alpha1.Kyma,
	modules []v1alpha1.Module,
) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, client, kyma.Name, kyma.Namespace)
		if err != nil {
			return err
		}
		kyma.Spec.Modules = modules
		return client.Update(ctx, kyma)
	}
}

func UpdateKymaLabel(ctx context.Context,
	client client.Client,
	kyma *v1alpha1.Kyma,
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

func ModuleNotExist(ctx context.Context, kyma *v1alpha1.Kyma, module v1alpha1.Module) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		if err != nil {
			return err
		}
		_, err = getModule(kyma, module)
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}
}

func SKRModuleExistWithOverwrites(kyma *v1alpha1.Kyma, module v1alpha1.Module) string {
	kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
	Expect(err).ToNot(HaveOccurred())
	moduleInCluster, err := getModule(kyma, module)
	Expect(err).ToNot(HaveOccurred())
	manifestSpec := moduleInCluster.Spec
	body, err := json.Marshal(manifestSpec.Resource.Object["spec"])
	Expect(err).ToNot(HaveOccurred())
	skrModuleSpec := sampleCRDv1alpha1.SKRModuleSpec{}
	err = json.Unmarshal(body, &skrModuleSpec)
	Expect(err).ToNot(HaveOccurred())
	return skrModuleSpec.InitKey
}

func getModule(kyma *v1alpha1.Kyma, module v1alpha1.Module) (*v1alpha1.Manifest, error) {
	for _, moduleStatus := range kyma.Status.Modules {
		if moduleStatus.Name == module.Name {
			component := &v1alpha1.Manifest{}
			err := controlPlaneClient.Get(ctx, client.ObjectKey{
				Namespace: moduleStatus.Manifest.GetNamespace(),
				Name:      moduleStatus.Manifest.GetName(),
			}, component)
			if err != nil {
				return nil, err
			}
			return component, nil
		}
	}
	return nil, fmt.Errorf("no module status mapping exists for module %s: %w", module.Name,
		k8serrors.NewNotFound(v1alpha1.GroupVersionResource.GroupResource(), module.Name))
}

func GetModuleTemplate(name string) (*v1alpha1.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1alpha1.ModuleTemplate{}
	moduleTemplateInCluster.SetNamespace(metav1.NamespaceDefault)
	moduleTemplateInCluster.SetName(name)
	err := getModuleTemplate(controlPlaneClient, moduleTemplateInCluster, nil, false)
	if err != nil {
		return nil, err
	}
	return moduleTemplateInCluster, nil
}

func KymaExists(remoteClient client.Client, name, namespace string) func() error {
	return func() error {
		_, err := GetKyma(ctx, remoteClient, name, namespace)
		return err
	}
}

func ModuleTemplatesExist(clnt client.Client, kyma *v1alpha1.Kyma, remote bool) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			if err != nil {
				return err
			}
			if err := getModuleTemplate(clnt, template, kyma, remote); err != nil {
				return err
			}
		}

		return nil
	}
}

var ErrModuleTemplateDescriptorLabelCountMismatch = errors.New("label count in descriptor does not match")

func ModuleTemplatesLabelsCountMatch(
	clnt client.Client, kyma *v1alpha1.Kyma, count int, remote bool,
) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			if err != nil {
				return err
			}
			if err := getModuleTemplate(clnt, template, kyma, remote); err != nil {
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

func ModifyModuleTemplateSpecThroughLabels(
	clnt client.Client,
	kyma *v1alpha1.Kyma,
	labels []ocm.Label,
	remote bool,
) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
			if err != nil {
				return err
			}

			if err := getModuleTemplate(clnt, template, kyma, remote); err != nil {
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

func getModuleTemplate(clnt client.Client, template *v1alpha1.ModuleTemplate, kyma *v1alpha1.Kyma, remote bool) error {
	if remote && kyma.Spec.Sync.Namespace != "" {
		template.SetNamespace(kyma.Spec.Sync.Namespace)
	}
	return clnt.Get(ctx, client.ObjectKeyFromObject(template), template)
}

func deleteModule(kyma *v1alpha1.Kyma, module v1alpha1.Module) func() error {
	return func() error {
		component, err := getModule(kyma, module)
		if k8serrors.IsNotFound(err) {
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
	if err := controlPlaneClient.Update(ctx, kyma); err != nil {
		return err
	}
	return nil
}

var ErrTemplateInfoChannelMismatch = errors.New("mismatch in template info channel")

func TemplateInfosMatchChannel(kymaName, channel string) error {
	kyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return err
	}
	for i := range kyma.Status.Modules {
		if kyma.Status.Modules[i].Channel != channel {
			return fmt.Errorf("%w: %s should be %s",
				ErrTemplateInfoChannelMismatch, kyma.Status.Modules[i].Channel, channel)
		}
	}
	return nil
}

func CreateModuleTemplateSetsForKyma(modules []v1alpha1.Module, modifiedVersion, channel string) error {
	for _, module := range modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{})
		if err != nil {
			return err
		}
		if err := template.Spec.ModifyDescriptor(
			v1alpha1.ModifyDescriptorVersion(
				func(version *semver.Version) string {
					return modifiedVersion
				},
			),
		); err != nil {
			return err
		}
		template.Spec.Channel = channel
		template.Name = fmt.Sprintf("%s-%s", template.Name, channel)
		if err := controlPlaneClient.Create(ctx, template); err != nil {
			return err
		}
	}
	return nil
}
