package api

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	runtimeScheme "sigs.k8s.io/controller-runtime/pkg/scheme"
)

func AddToScheme(scheme *machineryruntime.Scheme) error {

	if err := RequiredSchemeBuilder().AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add scheme on v1beta2 api: %w", err)
	}

	return nil
}

func RequiredSchemeBuilder() *runtimeScheme.Builder {
	return &runtimeScheme.Builder{GroupVersion: v1beta2.GroupVersion}
}

func InitCRD() {
	SchemeBuilder := &runtimeScheme.Builder{GroupVersion: v1beta2.GroupVersion}
	SchemeBuilder.Register(&v1beta2.Kyma{}, &v1beta2.KymaList{})
	SchemeBuilder.Register(&v1beta2.ModuleTemplate{}, &v1beta2.ModuleTemplateList{})
	SchemeBuilder.Register(&v1beta2.Watcher{}, &v1beta2.WatcherList{})
	SchemeBuilder.Register(&v1beta2.Manifest{}, &v1beta2.ManifestList{})
	SchemeBuilder.Register(&v1beta2.ModuleReleaseMeta{}, &v1beta2.ModuleReleaseMetaList{})
}
