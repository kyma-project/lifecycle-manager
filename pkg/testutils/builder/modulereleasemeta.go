package builder

import (
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ModuleReleaseMetaBuilder struct {
	moduleReleaseMeta *v1beta2.ModuleReleaseMeta
}

func NewModuleReleaseMetaBuilder() ModuleReleaseMetaBuilder {
	return ModuleReleaseMetaBuilder{
		moduleReleaseMeta: &v1beta2.ModuleReleaseMeta{
			TypeMeta: apimetav1.TypeMeta{
				APIVersion: v1beta2.GroupVersion.String(),
				Kind:       string(shared.ModuleReleaseMetaKind),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Namespace: apimetav1.NamespaceDefault,
			},
			Spec: v1beta2.ModuleReleaseMetaSpec{
				Channels: []v1beta2.ChannelVersionAssignment{},
			},
		},
	}
}

func (m ModuleReleaseMetaBuilder) WithName(name string) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.ObjectMeta.Name = name
	return m
}

func (m ModuleReleaseMetaBuilder) WithModuleChannelAndVersions(channelVersions []v1beta2.ChannelVersionAssignment) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Spec.Channels = append(m.moduleReleaseMeta.Spec.Channels, channelVersions...)
	return m
}

func (m ModuleReleaseMetaBuilder) WithModuleName(moduleName string) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Spec.ModuleName = moduleName
	m.moduleReleaseMeta.Name = moduleName

	return m
}

func (m ModuleReleaseMetaBuilder) Build() *v1beta2.ModuleReleaseMeta {
	return m.moduleReleaseMeta
}
