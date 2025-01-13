package maintenancewindows

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/maintenancewindows/resolver"
)

const (
	PolicyName        = "policy"
	PoliciesDirectory = "/etc/maintenance-policy"
)

func InitializeMaintenanceWindowsPolicy(log logr.Logger) (*resolver.MaintenanceWindowPolicy, error) {
	if err := os.Setenv(resolver.PolicyPathENV, PoliciesDirectory); err != nil {
		return nil, fmt.Errorf("failed to set the policy path env variable, %w", err)
	}

	policyFilePath := fmt.Sprintf("%s/%s.json", PoliciesDirectory, PolicyName)
	if !MaintenancePolicyFileExists(policyFilePath) {
		log.Info("maintenance windows policy file does not exist")
		return nil, nil //nolint:nilnil //use nil to indicate an empty Maintenance Window Policy
	}

	maintenancePolicyPool, err := resolver.GetMaintenancePolicyPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get maintenance policy pool, %w", err)
	}

	maintenancePolicy, err := resolver.GetMaintenancePolicy(maintenancePolicyPool, PolicyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get maintenance window policy, %w", err)
	}

	return maintenancePolicy, nil
}

func MaintenancePolicyFileExists(policyFilePath string) bool {
	if _, err := os.Stat(policyFilePath); os.IsNotExist(err) {
		return false
	}

	return true
}
