package builder

import (
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
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
	m.moduleReleaseMeta.Name = name
	return m
}

func (m ModuleReleaseMetaBuilder) WithSingleModuleChannelAndVersions(channel, version string) ModuleReleaseMetaBuilder {
	chanVer := v1beta2.ChannelVersionAssignment{Channel: channel, Version: version}
	m.moduleReleaseMeta.Spec.Channels = append(m.moduleReleaseMeta.Spec.Channels, chanVer)
	return m
}

func (m ModuleReleaseMetaBuilder) WithModuleChannelAndVersions(
	channelVersions []v1beta2.ChannelVersionAssignment,
) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Spec.Channels = append(m.moduleReleaseMeta.Spec.Channels, channelVersions...)
	return m
}

func (m ModuleReleaseMetaBuilder) WithModuleName(moduleName string) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Spec.ModuleName = moduleName
	m.moduleReleaseMeta.Name = moduleName

	return m
}

func (m ModuleReleaseMetaBuilder) WithOcmComponentName(ocmComponentName string) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Spec.OcmComponentName = ocmComponentName
	return m
}

func (m ModuleReleaseMetaBuilder) WithNamespace(namespace string) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Namespace = namespace
	return m
}

func (m ModuleReleaseMetaBuilder) WithMandatory(version string) ModuleReleaseMetaBuilder {
	m.moduleReleaseMeta.Spec.Mandatory = &v1beta2.Mandatory{Version: version}
	return m
}

func (m ModuleReleaseMetaBuilder) Build() *v1beta2.ModuleReleaseMeta {
	return m.moduleReleaseMeta
}
