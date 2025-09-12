package cache

import (
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

type DescriptorKey string

func GenerateDescriptorKey(ocmi ocmidentity.Component) DescriptorKey {
	return DescriptorKey(fmt.Sprintf("%s:%s", ocmi.Name(), ocmi.Version()))
}
