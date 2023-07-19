package v1beta2

import "errors"

var (
	ErrTypeAssertKyma           = errors.New("raw can not be cast to v1beta2.Kyma")
	ErrTypeAssertManifest       = errors.New("raw can not be cast to v1beta2.Manifest")
	ErrTypeAssertModuleTemplate = errors.New("raw can not be cast to v1beta2.ModuleTemplate")
	ErrTypeAssertWatcher        = errors.New("raw can not be cast to v1beta2.Watcher")
	ErrTypeAssertDescriptor     = errors.New("raw can not be cast to v1beta2.Descriptor")
)
