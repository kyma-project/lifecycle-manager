package cache

import (
	"fmt"

	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

type DescriptorKey string

func GenerateDescriptorKey(ocmId ocmidentity.ComponentId) DescriptorKey {
	return DescriptorKey(fmt.Sprintf("%s:%s", ocmId.Name(), ocmId.Version()))
}
