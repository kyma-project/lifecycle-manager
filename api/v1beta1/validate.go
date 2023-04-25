package v1beta1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateInstalls(manifest *Manifest) field.ErrorList {
	fieldErrors := make(field.ErrorList, 0)
	codec, err := v1beta2.NewCodec()
	if err != nil {
		fieldErrors = append(
			fieldErrors,
			field.Invalid(
				field.NewPath("spec").Child("installs"),
				"validator initialize", err.Error(),
			),
		)
	}

	if len(fieldErrors) == 0 {
		specType, err := v1beta2.GetSpecType(manifest.Spec.Install.Raw())
		if err != nil {
			fieldErrors = append(
				fieldErrors,
				field.Invalid(
					field.NewPath("spec").Child("installs"),
					manifest.Spec.Install.Raw(), err.Error(),
				),
			)
			return fieldErrors
		}

		err = codec.Validate(manifest.Spec.Install.Raw(), specType)
		if err != nil {
			fieldErrors = append(
				fieldErrors,
				field.Invalid(
					field.NewPath("spec").Child("installs"),
					manifest.Spec.Install.Raw(), err.Error(),
				),
			)
		}
	}

	if len(fieldErrors) > 0 {
		return fieldErrors
	}

	return nil
}
