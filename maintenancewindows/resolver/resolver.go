package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	policyPathENV = "MAINTENANCE_POLICY_PATH"
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

// GetMaintenancePolicyPool extracts and returns the maintenance policies we have under the policy directory
func GetMaintenancePolicyPool() (map[string]*[]byte, error) {
	pool := map[string]*[]byte{}

	path := os.Getenv(policyPathENV)
	if path == "" {
		return nil, fmt.Errorf("no environment variable set for the maintenance policy path")
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("error while reading directory %s: %v", path, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			return nil, fmt.Errorf("error while reading the file %s: %w", name, err)
		}

		pool[name] = &data
	}

	return pool, nil
}
