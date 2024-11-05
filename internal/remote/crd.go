package remote

import (
	"errors"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
)

func crdReady(crd *apiextensionsv1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextensionsv1.Established &&
			cond.Status == apiextensionsv1.ConditionTrue {
			return true
		}

		if cond.Type == apiextensionsv1.NamesAccepted &&
			cond.Status == apiextensionsv1.ConditionFalse {
			// This indicates a naming conflict, but it's probably not the
			// job of this function to fail because of that. Instead,
			// we treat it as a success, since the process should be able to
			// continue.
			return true
		}
	}
	return false
}

func containsCRDNotFoundError(errs []error) bool {
	for _, err := range errs {
		unwrappedError := errors.Unwrap(err)
		if meta.IsNoMatchError(unwrappedError) || CRDNotFoundErr(unwrappedError) {
			return true
		}
	}
	return false
}
