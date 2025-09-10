package ocmidentity

import (
	"errors"
	"fmt"
)

var (
	ErrValueNotProvided = errors.New("value not provided")
)

// Component uniquely identifies an OCM Component.
// See: https://ocm.software/docs/overview/important-terms/#component-identity
type Component struct {
	componentName    string
	componentVersion string
}

func (c *Component) Name() string {
	return c.componentName
}

func (c *Component) Version() string {
	return c.componentVersion
}

// New is a constructor that ensures that both name and version are provided.
func New(name, version string) (*Component, error) {
	if name == "" {
		return nil, fmt.Errorf("invalid component name: %w", ErrValueNotProvided)
	}
	if version == "" {
		return nil, fmt.Errorf("invalid component version: %w", ErrValueNotProvided)
	}

	return &Component{
		componentName:    name,
		componentVersion: version,
	}, nil
}
