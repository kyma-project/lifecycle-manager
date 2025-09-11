package cache

import (
	"fmt"
)

type DescriptorKey string

/*
TODO: Remove
func GenerateDescriptorKey(template *v1beta2.ModuleTemplate) DescriptorKey {
	version, err := template.GetSemanticVersion()
	if err == nil {
		return DescriptorKey(fmt.Sprintf("%s:%s:%d:%s", template.Name, template.Spec.Channel, template.Generation,
			version))
	}

	return DescriptorKey(fmt.Sprintf("%s:%s:%d", template.Name, template.Spec.Channel, template.Generation))
}
*/

func GenerateDescriptorKey(ociComponentName, componentVersion string) DescriptorKey {
	return DescriptorKey(fmt.Sprintf("%s:%s", ociComponentName, componentVersion))
}
