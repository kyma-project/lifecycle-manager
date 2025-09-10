package ocmidentity

import (
	"errors"
	"fmt"
)

var ErrValueNotProvided = errors.New("value not provided")

// Component uniquely identifies an OCM Component.
// See: https://ocm.software/docs/overview/important-terms/#component-identity
type Component struct {
	componentName    string
	componentVersion string
}

// MustNew is a convenience constructor that panics if name or version are not provided.
func MustNew(name, version string) *Component {
	c, err := New(name, version)
	if err != nil {
		panic(err)
	}
	return c
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

func (c *Component) Name() string {
	return c.componentName
}

func (c *Component) Version() string {
	return c.componentVersion
}
