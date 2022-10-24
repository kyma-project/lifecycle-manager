package test_helper

import (
	"math/rand"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	namespace = "default"
)
const letterBytes = "abcdefghijklmnopqrstuvwxyz"

func NewTestKyma(name string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: v1.TypeMeta{
			APIVersion: v1alpha1.GroupVersion.String(),
			Kind:       string(v1alpha1.KymaKind),
		},
		ObjectMeta: v1.ObjectMeta{
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

func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))] //nolint:gosec
	}
	return string(b)
}
