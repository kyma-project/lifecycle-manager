package v1beta1

import "k8s.io/apimachinery/pkg/util/validation/field"

func ValidateInstalls(manifest *Manifest) field.ErrorList {
	fieldErrors := make(field.ErrorList, 0)
	codec, err := NewCodec()
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
		for _, install := range manifest.Spec.Installs {
			specType, err := GetSpecType(install.Raw())
			if err != nil {
				fieldErrors = append(
					fieldErrors,
					field.Invalid(
						field.NewPath("spec").Child("installs"),
						install.Raw(), err.Error(),
					),
				)
				continue
			}

			err = codec.Validate(install.Raw(), specType)
			if err != nil {
				fieldErrors = append(
					fieldErrors,
					field.Invalid(
						field.NewPath("spec").Child("installs"),
						install.Raw(), err.Error(),
					),
				)
			}
		}
	}

	if len(fieldErrors) > 0 {
		return fieldErrors
	}

	return nil
}
