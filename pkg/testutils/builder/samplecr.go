package builder

import (
	"github.com/kyma-project/template-operator/api/v1alpha1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SampleCRBuilder struct {
	sample *v1alpha1.Sample
}

func NewSampleCRBuilder() SampleCRBuilder {
	return SampleCRBuilder{&v1alpha1.Sample{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Sample",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name: "sample-cr",
		},
		Spec: v1alpha1.SampleSpec{
			ResourceFilePath: "/some/resource/filepath",
		},
	}}
}

func (sb SampleCRBuilder) WithName(name string) SampleCRBuilder {
	sb.sample.SetName(name)
	return sb
}

func (sb SampleCRBuilder) WithNamespace(namespace string) SampleCRBuilder {
	sb.sample.SetNamespace(namespace)
	return sb
}

func (sb SampleCRBuilder) Build() *v1alpha1.Sample {
	return sb.sample
}
