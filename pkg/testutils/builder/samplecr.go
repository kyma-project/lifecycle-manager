package builder

import (
	"github.com/kyma-project/template-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SampleCRBuilder struct {
	sample *v1alpha1.Sample
}

func NewSampleCRBuilder() SampleCRBuilder {
	return SampleCRBuilder{&v1alpha1.Sample{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Sample",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "sample-cr",
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

//func (sb SampleCRBuilder) WithSpec(key, value string) SampleCRBuilder {
//	err := unstructured.SetNestedField(sb.sample.Spec, value, "spec", key)
//	if err != nil {
//		panic(fmt.Errorf("default cr: %w", err))
//	}
//	return sb
//}

func (sb SampleCRBuilder) Build() *v1alpha1.Sample {
	return sb.sample
}
