package builder

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ModuleCRBuilder struct {
	moduleCR *unstructured.Unstructured
}

func NewModuleCRBuilder() ModuleCRBuilder {
	moduleCR := &unstructured.Unstructured{}
	moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   v1beta2.GroupVersion.Group,
			Version: "v1alpha1",
			Kind:    "Sample",
		},
	)
	builder := ModuleCRBuilder{moduleCR: moduleCR}
	return builder
}

func (cr ModuleCRBuilder) WithGroupVersionKind(group, version, kind string) ModuleCRBuilder {
	cr.moduleCR.SetGroupVersionKind(
		schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		},
	)
	return cr
}

func (cr ModuleCRBuilder) WithName(name string) ModuleCRBuilder {
	cr.moduleCR.SetName(name)
	return cr
}

func (cr ModuleCRBuilder) WithNamespace(namespace string) ModuleCRBuilder {
	cr.moduleCR.SetNamespace(namespace)
	return cr
}

func (cr ModuleCRBuilder) WithSpec(key, value string) ModuleCRBuilder {
	err := unstructured.SetNestedField(cr.moduleCR.Object, value, "spec", key)
	if err != nil {
		panic(fmt.Errorf("default cr: %w", err))
	}
	return cr
}

func (cr ModuleCRBuilder) Build() *unstructured.Unstructured {
	return cr.moduleCR
}
