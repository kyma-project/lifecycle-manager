package builder

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const group = "operator.kyma-project.io"

type CRDBuilder struct {
	crd *apiextensions.CustomResourceDefinition
}

// NewCRDBuilder returns a CRDBuilder for CustomResourceDefinitions of Group operator.kyma-project.io initialized with a random name
func NewCRDBuilder() CRDBuilder {
	crdName := testutils.RandomName()

	return CRDBuilder{
		crd: &apiextensions.CustomResourceDefinition{
			TypeMeta: metav1.TypeMeta{
				Kind:       "CustomResourceDefinition",
				APIVersion: "apiextensions.k8s.io/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
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

// WithName sets metav1.ObjectMeta.Name and all apiextensions.CustomResourceDefinitionNames
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
