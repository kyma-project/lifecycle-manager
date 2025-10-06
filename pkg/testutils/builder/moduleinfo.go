package builder

import (
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
)

type ModuleInfoBuilder struct {
	moduleInfo *templatelookup.ModuleInfo
}

func NewModuleInfoBuilder() ModuleInfoBuilder {
	return ModuleInfoBuilder{
		moduleInfo: &templatelookup.ModuleInfo{
			Module: v1beta2.Module{},
		},
	}
}

func (b ModuleInfoBuilder) WithName(name string) ModuleInfoBuilder {
	b.moduleInfo.Name = name
	return b
}

func (b ModuleInfoBuilder) WithVersion(version string) ModuleInfoBuilder {
	b.moduleInfo.Version = version
	return b
}

func (b ModuleInfoBuilder) WithChannel(channel string) ModuleInfoBuilder {
	b.moduleInfo.Channel = channel
	return b
}

func (b ModuleInfoBuilder) Enabled() ModuleInfoBuilder {
	b.moduleInfo.Enabled = true
	return b
}

func (b ModuleInfoBuilder) Build() *templatelookup.ModuleInfo {
	return b.moduleInfo
}
