package v1beta2

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&v1beta2.Kyma{}, &v1beta2.KymaList{})
	SchemeBuilder.Register(&v1beta2.Manifest{}, &v1beta2.ManifestList{})
	SchemeBuilder.Register(&v1beta2.Watcher{}, &v1beta2.WatcherList{})
	SchemeBuilder.Register(&v1beta2.ModuleTemplate{}, &v1beta2.ModuleTemplateList{}, &v1beta2.Descriptor{})
}

type KymaInCtrlRuntime struct {
	*v1beta2.Kyma
}

type ManifestInCtrlRuntime struct {
	*v1beta2.Manifest
}

type WatcherInCtrlRuntime struct {
	*v1beta2.Watcher
}

type ModuleTemplateInCtrlRuntime struct {
	*v1beta2.ModuleTemplate
}
