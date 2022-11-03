//go:build testing

package testhelper

import (
	"context"
	"math/rand"
	"time"

	"github.com/kyma-project/lifecycle-manager/operator/pkg/test"

	//nolint:stylecheck,revive
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	randomStringLength = 8
	letterBytes        = "abcdefghijklmnopqrstuvwxyz"
	Timeout            = time.Second * 10
	Interval           = time.Millisecond * 250
)

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       string(v1alpha1.KymaKind),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + RandString(randomStringLength),
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.KymaSpec{
			Modules: []v1alpha1.Module{},
			Channel: v1alpha1.DefaultChannel,
		},
	}
}

func NewUniqModuleName() string {
	return RandString(randomStringLength)
}

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec
	}
	return string(b)
}

func DeployModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1alpha1.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(kcpClient.Create(ctx, template)).To(Succeed())
	}
}

func DeleteModuleTemplates(ctx context.Context, kcpClient client.Client, kyma *v1alpha1.Kyma) {
	for _, module := range kyma.Spec.Modules {
		template, err := test.ModuleTemplateFactory(module, unstructured.Unstructured{})
		Expect(err).ShouldNot(HaveOccurred())
		Expect(kcpClient.Delete(ctx, template)).To(Succeed())
	}
}

func GetKyma(ctx context.Context, testClient client.Client, kymaName string) (*v1alpha1.Kyma, error) {
	kymaInCluster := &v1alpha1.Kyma{}
	err := testClient.Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceDefault,
		Name:      kymaName,
	}, kymaInCluster)
	if err != nil {
		return nil, err
	}
	return kymaInCluster, nil
}

func IsKymaInState(ctx context.Context, kcpClient client.Client, kymaName string, state v1alpha1.State) func() bool {
	return func() bool {
		kymaFromCluster, err := GetKyma(ctx, kcpClient, kymaName)
		if err != nil || kymaFromCluster.Status.State != state {
			return false
		}
		return true
	}
}
