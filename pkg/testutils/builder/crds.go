package builder

import (
	"fmt"
	"strings"
	"unicode"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const group = "operator.kyma-project.io"

type CRDBuilder struct {
	crd *apiextensions.CustomResourceDefinition
}

// NewCRDBuilder returns a CRDBuilder for CustomResourceDefinitions of Group
// operator.kyma-project.io initialized with a random name.
func NewCRDBuilder() CRDBuilder {
	crdName := RandomName()

	return CRDBuilder{
		crd: &apiextensions.CustomResourceDefinition{
			TypeMeta: apimachinerymeta.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: apimachinerymeta.ObjectMeta{
				Name: fmt.Sprintf("%ss.%s", strings.ToLower(crdName), group),
			},
			Spec: apiextensions.CustomResourceDefinitionSpec{
				Group: group,
				Names: createCRDNamesFrom(crdName),
				Scope: "Namespaced",
			},
		},
	}
}

// WithName sets ObjectMeta.Name and all apiextensions.CustomResourceDefinitionNames.
func (cb CRDBuilder) WithName(name string) CRDBuilder {
	cb.crd.Name = fmt.Sprintf("%ss.%s", strings.ToLower(name), group)
	cb.crd.Spec.Names = createCRDNamesFrom(name)
	return cb
}

// Build returns the apiextensions.CustomResourceDefinition from the Builder.
func (cb CRDBuilder) Build() apiextensions.CustomResourceDefinition {
	return *cb.crd
}

func createCRDNamesFrom(s string) apiextensions.CustomResourceDefinitionNames {
	name := strings.ToLower(s)
	return apiextensions.CustomResourceDefinitionNames{
		Kind:     upperCaseFirst(name),
		ListKind: upperCaseFirst(name) + "List",
		Plural:   name + "s",
		Singular: name,
	}
}

func upperCaseFirst(s string) string {
	if len(s) < 1 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
