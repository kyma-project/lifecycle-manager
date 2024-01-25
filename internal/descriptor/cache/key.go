package cache

import (
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type DescriptorCacheKey string

func GenerateDescriptorCacheKey(template *v1beta2.ModuleTemplate) DescriptorCacheKey {
	if template.Annotations != nil {
		moduleVersion := template.Annotations[shared.ModuleVersionAnnotation]
		_, err := semver.NewVersion(moduleVersion)
		if moduleVersion != "" && err == nil {
			return DescriptorCacheKey(fmt.Sprintf("%s:%s:%s", template.Name, template.Spec.Channel, moduleVersion))
		}
	}

	return DescriptorCacheKey(fmt.Sprintf("%s:%s:%d", template.Name, template.Spec.Channel, template.Generation))
}
