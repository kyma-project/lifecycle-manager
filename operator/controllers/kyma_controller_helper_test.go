package controllers_test

import (
	"encoding/json"
	"math/rand"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	sampleCRDv1alpha1 "github.com/kyma-project/lifecycle-manager/operator/config/samples/component-integration-installed/crd/v1alpha1" //nolint:lll
	"github.com/kyma-project/lifecycle-manager/operator/controllers"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/watch"
	manifestV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
)

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       string(v1alpha1.KymaKind),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + RandString(8),
			Namespace: namespace,
		},
		Spec: v1alpha1.KymaSpec{
			Modules: []v1alpha1.Module{},
			Channel: v1alpha1.DefaultChannel,
		},
	}
}

func NewUniqModuleName() string {
	return RandString(8)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec
	}
	return string(b)
}

func RegisterDefaultLifecycleForKyma(kyma *v1alpha1.Kyma) {
	BeforeEach(func() {
		Expect(controlPlaneClient.Create(ctx, kyma)).Should(Succeed())
	})
	AfterEach(func() {
		Expect(controlPlaneClient.Delete(ctx, kyma)).Should(Succeed())
	})
}

func IsKymaInState(kymaName string, state v1alpha1.State) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil || kymaFromCluster.Status.State != state {
			return false
		}
		return true
	}
}

func GetKymaState(kymaName string) func() string {
	return func() string {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
		if err != nil {
			return ""
		}
		return string(createdKyma.Status.State)
	}
}

func GetKymaConditions(kymaName string) func() []metav1.Condition {
	return func() []metav1.Condition {
		createdKyma, err := GetKyma(controlPlaneClient, kymaName)
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

func getModule(kymaName, moduleName string) (*unstructured.Unstructured, error) {
	component := &unstructured.Unstructured{}
	component.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   v1alpha1.OperatorPrefix,
		Version: v1alpha1.Version,
		Kind:    "Manifest",
	})
	err := controlPlaneClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      common.CreateModuleName(moduleName, kymaName),
	}, component)
	if err != nil {
		return nil, err
	}
	return component, nil
}

func GetKyma(
	testClient client.Client,
	kymaName string,
) (*v1alpha1.Kyma, error) {
	kymaInCluster := &v1alpha1.Kyma{}
	err := testClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      kymaName,
	}, kymaInCluster)
	if err != nil {
		return nil, err
	}
	return kymaInCluster, nil
}

func RemoteKymaExists(remoteClient client.Client, kymaName string) func() error {
	return func() error {
		_, err := GetKyma(remoteClient, kymaName)
		return err
	}
}

func getCatalog(
	clnt client.Client,
	kyma *v1alpha1.Kyma,
) (*v1.ConfigMap, error) {
	catalog := &v1.ConfigMap{}
	catalog.SetName(controllers.CatalogName)
	catalog.SetNamespace(kyma.GetNamespace())
	err := clnt.Get(ctx, client.ObjectKeyFromObject(catalog), catalog)
	if err != nil {
		return nil, err
	}
	return catalog, nil
}

func CatalogExists(clnt client.Client, kyma *v1alpha1.Kyma) func() error {
	return func() error {
		_, err := getCatalog(clnt, kyma)
		return err
	}
}

func deleteModule(kyma *v1alpha1.Kyma, moduleTemplate *v1alpha1.ModuleTemplate,
) error {
	component := moduleTemplate.Spec.Data.DeepCopy()
	if moduleTemplate.Spec.Target == v1alpha1.TargetRemote {
		component.SetKind("Manifest")
	}
	component.SetNamespace(namespace)
	component.SetName(common.CreateModuleName(moduleTemplate.GetLabels()[v1alpha1.ModuleName], kyma.GetName()))
	return controlPlaneClient.Delete(ctx, component)
}
