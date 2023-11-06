package matcher

import (
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type CRDMatcherFunc func(crd apiextensionsv1.CustomResourceDefinition) bool

// CreateCRDMatcherFrom returns a CRDMatcherFunc for a comma-separated list of CRDs.
// Every CRD is defined using  the syntax: `<names.plural>.<group>` or `<names.singular>.<group>`,
// e.g:. "kymas.operator.kyma-project.io" or "kyma.operator.kyma-project.io"
// Instead of a name, an asterisk `*` may be used. It matches any name for the given group.
func CreateCRDMatcherFrom(input string) CRDMatcherFunc {
	trimmed := strings.TrimSpace(input)
	defs := strings.Split(trimmed, ",")
	if len(defs) == 0 {
		return emptyMatcher()
	}

	return crdMatcherForItems(defs)
}

func crdMatcherForItems(defs []string) CRDMatcherFunc {
	var matchers []CRDMatcherFunc
	for _, def := range defs {
		matcher := crdMatcherForItem(def)
		if matcher != nil {
			matchers = append(matchers, matcher)
		}
	}

	return func(crd apiextensionsv1.CustomResourceDefinition) bool {
		for _, doesMatch := range matchers {
			if doesMatch(crd) {
				return true
			}
		}
		return false
	}
}

// crdMatcherForItem returns a CRDMatcherFunc for a given CRD reference.
// The reference is expected to be in the form: `<names.plural>.<group>` or `<names.singular>.<group>`,
// e.g:. "kymas.operator.kyma-project.io" or "kyma.operator.kyma-project.io"
// Instead of a CRD name an asterisk `*` may be used, it matches any name for the given group.
// See the: k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1/CustomResourceDefinition type for details.
func crdMatcherForItem(givenCRDReference string) CRDMatcherFunc {
	nameSegments := strings.Split(givenCRDReference, ".")
	const minSegments = 2
	if len(nameSegments) < minSegments {
		return emptyMatcher()
	}

	givenKind := strings.TrimSpace(strings.ToLower(nameSegments[0]))
	givenGroup := strings.TrimSpace(strings.ToLower(strings.Join(nameSegments[1:], ".")))

	return func(crd apiextensionsv1.CustomResourceDefinition) bool {
		lKind := strings.ToLower(crd.Spec.Names.Kind)
		lSingular := strings.ToLower(crd.Spec.Names.Singular)
		lPlural := strings.ToLower(crd.Spec.Names.Plural)
		lGroup := strings.ToLower(crd.Spec.Group)

		if givenGroup != lGroup {
			return false
		}

		return givenKind == "*" || givenKind == lPlural || givenKind == lSingular || givenKind == lKind
	}
}

func emptyMatcher() CRDMatcherFunc {
	return func(crd apiextensionsv1.CustomResourceDefinition) bool {
		return false
	}
}
