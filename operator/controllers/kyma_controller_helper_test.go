package controllers_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	sampleCRDv1alpha1 "github.com/kyma-project/kyma-operator/operator/config/samples/component-integration-installed/crd/v1alpha1" //nolint:lll
	"github.com/kyma-project/kyma-operator/operator/controllers"
	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	manifestV1alpha1 "github.com/kyma-project/manifest-operator/operator/api/v1alpha1"
)

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       string(v1alpha1.KymaKind),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.KymaSpec{
			Modules: []v1alpha1.Module{},
			Channel: v1alpha1.DefaultChannel,
			Profile: v1alpha1.DefaultProfile,
		},
	}
}

func RegisterDefaultLifecycleForKyma(kyma *v1alpha1.Kyma) {
	BeforeEach(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})
}

func IsKymaInState(kyma *v1alpha1.Kyma, state v1alpha1.State) func() bool {
	return func() bool {
		kymaFromCluster := &v1alpha1.Kyma{}
		err := controlPlaneClient.Get(ctx, types.NamespacedName{
			Name:      kyma.GetName(),
			Namespace: kyma.GetNamespace(),
		}, kymaFromCluster)
		if err != nil || kymaFromCluster.Status.State != state {
			return false
		}

		return true
	}
}

func GetKymaState(kyma *v1alpha1.Kyma) func() string {
	return func() string {
		createdKyma := &v1alpha1.Kyma{}
		err := controlPlaneClient.Get(ctx, types.NamespacedName{
			Name: kyma.GetName(), Namespace: kyma.GetNamespace(),
		}, createdKyma)
		if err != nil {
			return ""
		}
		return string(createdKyma.Status.State)
	}
}

func GetKymaConditions(kyma *v1alpha1.Kyma) func() []v1alpha1.KymaCondition {
	return func() []v1alpha1.KymaCondition {
		createdKyma := &v1alpha1.Kyma{}
		err := controlPlaneClient.Get(ctx, types.NamespacedName{
			Name: kyma.GetName(), Namespace: kyma.GetNamespace(),
		}, createdKyma)
		if err != nil {
			return []v1alpha1.KymaCondition{}
		}
		return createdKyma.Status.Conditions
	}
}

func UpdateModuleState(
	kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate, state v1alpha1.State,
) func() error {
	return func() error {
		component, err := getModule(kyma, moduleTemplate)
		Expect(err).ShouldNot(HaveOccurred())
		component.Object[watch.Status] = map[string]any{watch.State: string(state)}
		return k8sManager.GetClient().Status().Update(ctx, component)
	}
}

func ModuleExist(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate) func() error {
	return func() error {
		_, err := getModule(kyma, moduleTemplate)
		return err
	}
}

func SKRModuleExistWithOverwrites(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate) func() string {
	return func() string {
		module, err := getModule(kyma, moduleTemplate)
		Expect(err).ToNot(HaveOccurred())
		body, err := json.Marshal(module.Object["spec"])
		Expect(err).ToNot(HaveOccurred())
		manifestSpec := manifestV1alpha1.ManifestSpec{}
		err = json.Unmarshal(body, &manifestSpec)
		Expect(err).ToNot(HaveOccurred())
		body, err = json.Marshal(manifestSpec.Resource.Object["spec"])
		Expect(err).ToNot(HaveOccurred())
		skrModuleSpec := sampleCRDv1alpha1.SKRModuleSpec{}
		err = json.Unmarshal(body, &skrModuleSpec)
		Expect(err).ToNot(HaveOccurred())
		return skrModuleSpec.InitKey
	}
}

func getModule(
	kyma *v1alpha1.Kyma,
	moduleTemplate *v1alpha1.ModuleTemplate,
) (*unstructured.Unstructured, error) {
	component := moduleTemplate.Spec.Data.DeepCopy()
	if moduleTemplate.Spec.Target == v1alpha1.TargetRemote {
		component.SetKind("Manifest")
	}
	err := controlPlaneClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      parsed.CreateModuleName(moduleTemplate.GetLabels()[v1alpha1.ModuleName], kyma.GetName()),
	}, component)
	if err != nil {
		return nil, err
	}
	return component, nil
}

func getRemoteKyma(
	remoteClient client.Client,
	kyma *v1alpha1.Kyma,
) (*v1alpha1.Kyma, error) {
	remoteKyma := &v1alpha1.Kyma{}
	err := remoteClient.Get(ctx, client.ObjectKeyFromObject(kyma), remoteKyma)
	if err != nil {
		return nil, err
	}
	return remoteKyma, nil
}

func RemoteKymaExists(remoteClient client.Client, kyma *v1alpha1.Kyma) func() error {
	return func() error {
		_, err := getRemoteKyma(remoteClient, kyma)
		return err
	}
}

func RemoteKyma(remoteClient client.Client, kyma *v1alpha1.Kyma, tester func(*v1alpha1.Kyma) error) func() error {
	return func() error {
		remoteKyma, err := getRemoteKyma(remoteClient, kyma)
		if err != nil {
			return err
		}
		return tester(remoteKyma)
	}
}

func getRemoteCatalog(
	remoteClient client.Client,
	kyma *v1alpha1.Kyma,
) (*v1.ConfigMap, error) {
	catalog := &v1.ConfigMap{}
	catalog.SetName(controllers.CatalogName)
	catalog.SetNamespace(kyma.GetNamespace())
	err := remoteClient.Get(ctx, client.ObjectKeyFromObject(catalog), catalog)
	if err != nil {
		return nil, err
	}
	return catalog, nil
}

func RemoteCatalogExists(remoteClient client.Client, kyma *v1alpha1.Kyma) func() error {
	return func() error {
		_, err := getRemoteCatalog(remoteClient, kyma)
		return err
	}
}
