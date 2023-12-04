package v1beta1

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

//nolint:gochecknoinits
func init() {
	SchemeBuilder.Register(&v1beta1.Kyma{}, &v1beta1.KymaList{})
	SchemeBuilder.Register(&v1beta1.Manifest{}, &v1beta1.ManifestList{})
	SchemeBuilder.Register(&v1beta1.Watcher{}, &v1beta1.WatcherList{})
	SchemeBuilder.Register(&v1beta1.ModuleTemplate{}, &v1beta1.ModuleTemplateList{}, &v1beta2.Descriptor{})
}

type KymaInCtrlRuntime struct {
	*v1beta1.Kyma
}

type ManifestInCtrlRuntime struct {
	*v1beta1.Manifest
}

type WatcherInCtrlRuntime struct {
	*v1beta1.Watcher
}

type ModuleTemplateInCtrlRuntime struct {
	*v1beta1.ModuleTemplate
}
