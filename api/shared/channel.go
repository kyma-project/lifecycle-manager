package shared

import "strings"

type Channel string

const (
	// NoneChannel when this value is defined for the ModuleTemplate,
	// it means that the ModuleTemplate is not assigned to any channel.
	NoneChannel Channel = "none"
)

func (c Channel) Equals(value string) bool {
	return string(c) == strings.ToLower(value)
}
