package resolver

import (
	"time"

)

// Runtime is the data type which captures the needed runtime specific attributes to perform orchestrations on a given runtime.
type Runtime struct {
	InstanceID             string
	RuntimeID              string
	GlobalAccountID        string
	SubAccountID           string
	ShootName              string
	Plan                   string
	Region                 string
	PlatformRegion         string
	MaintenanceWindowBegin time.Time
	MaintenanceWindowEnd   time.Time
	MaintenanceDays        []string
}
