package builder

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type DefaultCRBuilder struct {
	defaultCR *unstructured.Unstructured
}

const (
	InitSpecKey   = "initKey"
	InitSpecValue = "initValue"
)

func NewDefaultCRBuilder() DefaultCRBuilder {
	defaultCR := &unstructured.Unstructured{}
	defaultCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   v1beta2.GroupVersion.Group,
			Version: "v1alpha1",
			Kind:    "Sample",
		},
	)
	builder := DefaultCRBuilder{defaultCR: defaultCR}
	return builder
}

func (cr DefaultCRBuilder) WithGroupVersionKind(group, version, kind string) DefaultCRBuilder {
	cr.defaultCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		},
	)
	return cr
}

func (cr DefaultCRBuilder) WithName(name string) DefaultCRBuilder {
	cr.defaultCR.SetName(name)
	return cr
}

func (cr DefaultCRBuilder) WithNamespace(namespace string) DefaultCRBuilder {
	cr.defaultCR.SetNamespace(namespace)
	return cr
}

func (cr DefaultCRBuilder) WithSpec(key, value string) DefaultCRBuilder {
	err := unstructured.SetNestedField(cr.defaultCR.Object, value, "spec", key)
	if err != nil {
		panic(fmt.Errorf("default cr: %w", err))
	}
	return cr
}

func (cr DefaultCRBuilder) Build() *unstructured.Unstructured {
	return cr.defaultCR
}
