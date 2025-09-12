package ocmidentity

import (
	"errors"
	"fmt"
)

var (
	ErrMissingValue = errors.New("missing value")
)

// OCMComponentIdentity uniquely identifies an OCM Component.
// See: https://ocm.software/docs/overview/important-terms/#component-identity
type ComponentIdentity struct {
	ComponentName    string
	ComponentVersion string
}

// New is a helper function that ensures that both name and version are provided.
// This is important during the transition period as we still may have code paths
// relying just on ModuleTemplate which cannot provide the full OCM Component Name.
// Once we remove ModuleTemplate from module handling logic, these extra checks
// (or the whole function) can be removed.
func New(name, version string) (*ComponentIdentity, error) {
	if name == "" {
		return nil, fmt.Errorf("provided empty component name: %w", ErrMissingValue)
	}
	if version == "" {
		return nil, fmt.Errorf("provide empty component version: %w", ErrMissingValue)
	}

	return &ComponentIdentity{
		ComponentName:    name,
		ComponentVersion: version,
	}, nil
}
