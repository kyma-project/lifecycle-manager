package cache

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type DescriptorKey string

func GenerateDescriptorKey(template *v1beta2.ModuleTemplate) DescriptorKey {
	version, err := template.GetVersion()
	if err == nil {
		return DescriptorKey(fmt.Sprintf("%s:%s:%s", template.Name, template.Spec.Channel, version.String()))
	}

	return DescriptorKey(fmt.Sprintf("%s:%s:%d", template.Name, template.Spec.Channel, template.Generation))
}
