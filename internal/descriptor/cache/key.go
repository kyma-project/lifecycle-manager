package cache

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type DescriptorKey string

func GenerateDescriptorKey(template *v1beta2.ModuleTemplate) DescriptorKey {
	if template.Annotations != nil {
		moduleVersion := template.Annotations[shared.ModuleVersionAnnotation]
		_, err := semver.NewVersion(moduleVersion)
		if moduleVersion != "" && err == nil {
			return DescriptorKey(fmt.Sprintf("%s:%s:%d:%s", template.Name, template.Spec.Channel, template.Generation,
				moduleVersion))
		}
	}

	return DescriptorKey(fmt.Sprintf("%s:%s:%d", template.Name, template.Spec.Channel, template.Generation))
}
