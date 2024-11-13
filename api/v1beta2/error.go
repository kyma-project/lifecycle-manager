package v1beta2

import "errors"

var (
	ErrTypeAssertManifest       = errors.New("value can not be converted to v1beta2.Manifest")
	ErrTypeAssertModuleTemplate = errors.New("value can not be converted to v1beta2.ModuleTemplate")
)
