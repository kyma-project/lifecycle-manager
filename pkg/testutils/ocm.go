package testutils

import (
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
)

const DefaultFQDN = "kyma-project.io/module/template-operator"

// MustNewComponentId is a convenience ComponentId constructor that panics if name or version are not provided.
func MustNewComponentId(name, version string) *ocmidentity.ComponentId {
	ocmId, err := ocmidentity.NewComponentId(name, version)
	if err != nil {
		panic(err)
	}
	return ocmId
}
