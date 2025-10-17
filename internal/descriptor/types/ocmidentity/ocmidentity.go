package ocmidentity

import (
	"errors"
	"fmt"
)

var ErrValueNotProvided = errors.New("value not provided")

// ComponentId uniquely identifies an OCM ComponentId.
// See: https://ocm.software/docs/overview/important-terms/#component-identity
type ComponentId struct {
	componentName    string
	componentVersion string
}

// NewComponentId is a constructor that ensures that both name and version are provided.
func NewComponentId(name, version string) (*ComponentId, error) {
	if name == "" {
		return nil, fmt.Errorf("invalid component name: %w", ErrValueNotProvided)
	}
	if version == "" {
		return nil, fmt.Errorf("invalid component version: %w", ErrValueNotProvided)
	}

	return &ComponentId{
		componentName:    name,
		componentVersion: version,
	}, nil
}

func (c ComponentId) Name() string {
	return c.componentName
}

func (c ComponentId) Version() string {
	return c.componentVersion
}
