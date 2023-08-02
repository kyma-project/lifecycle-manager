package v1beta2

import "errors"

var (
	ErrTypeAssertKyma           = errors.New("value can not be converted to v1beta2.Kyma")
	ErrTypeAssertManifest       = errors.New("value can not be converted to v1beta2.Manifest")
	ErrTypeAssertModuleTemplate = errors.New("value can not be converted to v1beta2.ModuleTemplate")
	ErrTypeAssertWatcher        = errors.New("value can not be converted to v1beta2.Watcher")
	ErrTypeAssertDescriptor     = errors.New("value can not be converted to v1beta2.Descriptor")
)
