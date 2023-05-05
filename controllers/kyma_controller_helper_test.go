package controllers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	sampleCRDv1beta1 "github.com/kyma-project/lifecycle-manager/config/samples/component-integration-installed/crd/v1beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	ErrKymaNotFound             = errors.New("kyma not exists")
	ErrExpectedLabelNotReset    = errors.New("expected label not reset")
	ErrWatcherLabelMissing      = errors.New("watcher label missing")
	ErrWatcherAnnotationMissing = errors.New("watcher annotation missing")
)

func RegisterDefaultLifecycleForKyma(kyma *v1beta1.Kyma) {
	BeforeAll(func() {
		DeployModuleTemplates(ctx, controlPlaneClient, kyma, false)
	})

	AfterAll(func() {
		DeleteModuleTemplates(ctx, controlPlaneClient, kyma, false)
	})
	RegisterDefaultLifecycleForKymaWithoutTemplate(kyma)
}

func RegisterDefaultLifecycleForKymaWithoutTemplate(kyma *v1beta1.Kyma) {
	BeforeAll(func() {
		Eventually(controlPlaneClient.Create, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma).Should(Succeed())
	})

	AfterAll(func() {
		Eventually(controlPlaneClient.Delete, Timeout, Interval).
			WithContext(ctx).
			WithArguments(kyma).Should(Succeed())
	})

	BeforeEach(func() {
		By("get latest kyma CR")
		Eventually(SyncKyma, Timeout, Interval).WithArguments(kyma).Should(Succeed())
	})
}

func SyncKyma(kyma *v1beta1.Kyma) error {
	err := controlPlaneClient.Get(ctx, client.ObjectKey{
		Name:      kyma.Name,
		Namespace: metav1.NamespaceDefault,
	}, kyma)
	// It might happen in some test case, kyma get deleted, if you need to make sure Kyma should exist,
	// write expected condition to check it specifically.
	return client.IgnoreNotFound(err)
}

func GetKymaState(kymaName string) (string, error) {
	createdKyma, err := GetKyma(ctx, controlPlaneClient, kymaName, "")
	if err != nil {
		return "", err
	}
	return string(createdKyma.Status.State), nil
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
	ctx context.Context, kyma *v1beta1.Kyma, module v1beta1.Module, state v1beta1.State,
) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
		if err != nil {
			return err
		}
		return updateModuleState(kyma, module, state)
	}
}

func updateModuleState(kyma *v1beta1.Kyma, module v1beta1.Module, state v1beta1.State) error {
	component, err := getModule(kyma, module)
	if err != nil {
		return err
	}
	component.Status.State = declarative.State(state)
	return k8sManager.GetClient().Status().Update(ctx, component)
}

func ModuleExists(ctx context.Context, kyma *v1beta1.Kyma, module v1beta1.Module) func() error {
	return func() error {
		kyma, err := GetKyma(ctx, controlPlaneClient, kyma.Name, kyma.Namespace)
		if err != nil {
			return err
		}
		_, err = getModule(kyma, module)
		return err
	}
}

func UpdateRemoteModule(
	ctx context.Context,
	client client.Client,
	kyma *v1beta1.Kyma,
	modules []v1beta1.Module,
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

func UpdateKymaLabel(
	ctx context.Context,
	client client.Client,
	kyma *v1beta1.Kyma,
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

func ModuleNotExist(ctx context.Context, kyma *v1beta1.Kyma, module v1beta1.Module) func() error {
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

func SKRModuleExistWithOverwrites(kyma *v1beta1.Kyma, module v1beta1.Module) string {
	kyma, err := GetKyma(ctx, controlPlaneClient, kyma.GetName(), kyma.GetNamespace())
	Expect(err).ToNot(HaveOccurred())
	moduleInCluster, err := getModule(kyma, module)
	Expect(err).ToNot(HaveOccurred())
	manifestSpec := moduleInCluster.Spec
	body, err := json.Marshal(manifestSpec.Resource.Object["spec"])
	Expect(err).ToNot(HaveOccurred())
	skrModuleSpec := sampleCRDv1beta1.SKRModuleSpec{}
	err = json.Unmarshal(body, &skrModuleSpec)
	Expect(err).ToNot(HaveOccurred())
	return skrModuleSpec.InitKey
}

func getModule(kyma *v1beta1.Kyma, module v1beta1.Module) (*v1beta1.Manifest, error) {
	for _, moduleStatus := range kyma.Status.Modules {
		if moduleStatus.Name == module.Name {
			component := &v1beta1.Manifest{}
			err := controlPlaneClient.Get(
				ctx, client.ObjectKey{
					Namespace: moduleStatus.Manifest.GetNamespace(),
					Name:      moduleStatus.Manifest.GetName(),
				}, component,
			)
			if err != nil {
				return nil, err
			}
			return component, nil
		}
	}
	return nil, fmt.Errorf(
		"no module status mapping exists for module %s: %w", module.Name,
		k8serrors.NewNotFound(v1beta1.GroupVersionResource.GroupResource(), module.Name),
	)
}

func GetModuleTemplate(name string,
	clnt client.Client,
	kyma *v1beta1.Kyma,
	remote bool,
) (*v1beta1.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1beta1.ModuleTemplate{}
	moduleTemplateInCluster.SetNamespace(metav1.NamespaceDefault)
	moduleTemplateInCluster.SetName(name)
	if remote && kyma.Spec.Sync.Namespace != "" {
		moduleTemplateInCluster.SetNamespace(kyma.Spec.Sync.Namespace)
	}
	err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleTemplateInCluster), moduleTemplateInCluster)
	if err != nil {
		return nil, err
	}
	return moduleTemplateInCluster, nil
}

func KymaExists(clnt client.Client, name, namespace string) error {
	_, err := GetKyma(ctx, clnt, name, namespace)
	if k8serrors.IsNotFound(err) {
		return ErrKymaNotFound
	}
	return nil
}

func ModuleTemplatesExist(clnt client.Client, kyma *v1beta1.Kyma, remote bool) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			if _, err := GetModuleTemplate(module.Name, clnt, kyma, remote); err != nil {
				return err
			}
		}

		return nil
	}
}

func WatcherLabelsAnnotationsExist(clnt client.Client, kyma *v1beta1.Kyma) error {
	remoteKyma, err := GetKyma(ctx, clnt, kyma.GetName(), kyma.Spec.Sync.Namespace)
	if err != nil {
		return err
	}
	if remoteKyma.Labels[v1beta1.WatchedByLabel] != v1beta1.OperatorName {
		return ErrWatcherLabelMissing
	}
	if remoteKyma.Annotations[v1beta1.OwnedByAnnotation] != fmt.Sprintf(v1beta1.OwnedByFormat,
		kyma.GetNamespace(), kyma.GetName()) {
		return ErrWatcherAnnotationMissing
	}
	return nil
}

func deleteModule(kyma *v1beta1.Kyma, module v1beta1.Module) func() error {
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

func CreateModuleTemplateSetsForKyma(modules []v1beta1.Module, modifiedVersion, channel string) error {
	for _, module := range modules {
		template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, false)
		if err != nil {
			return err
		}

		descriptor, err := template.Spec.GetDescriptor()
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
