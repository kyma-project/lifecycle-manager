package maintenancewindows

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/maintenancewindows/resolver"
)

var ErrNoMaintenanceWindowPolicyConfigured = errors.New("no maintenance window policy configured")

type MaintenanceWindowPolicy interface {
	Resolve(runtime *resolver.Runtime, opts ...interface{}) (*resolver.ResolvedWindow, error)
}

type MaintenanceWindow struct {
	// make this private once we refactor the API
	// https://github.com/kyma-project/lifecycle-manager/issues/2190
	MaintenanceWindowPolicy MaintenanceWindowPolicy
}

func InitializeMaintenanceWindow(log logr.Logger,
	policiesDirectory, policyName string,
) (*MaintenanceWindow, error) {
	if err := os.Setenv(resolver.PolicyPathENV, policiesDirectory); err != nil {
		return nil, fmt.Errorf("failed to set the policy path env variable, %w", err)
	}

	policyFilePath := fmt.Sprintf("%s/%s.json", policiesDirectory, policyName)
	if !MaintenancePolicyFileExists(policyFilePath) {
		log.Info("maintenance windows policy file does not exist")
		return &MaintenanceWindow{
			MaintenanceWindowPolicy: nil,
		}, nil
	}

	maintenancePolicyPool, err := resolver.GetMaintenancePolicyPool()
	if err != nil {
		return nil, fmt.Errorf("failed to get maintenance policy pool, %w", err)
	}

	maintenancePolicy, err := resolver.GetMaintenancePolicy(maintenancePolicyPool, policyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get maintenance window policy, %w", err)
	}

	return &MaintenanceWindow{
		MaintenanceWindowPolicy: maintenancePolicy,
	}, nil
}

func MaintenancePolicyFileExists(policyFilePath string) bool {
	if _, err := os.Stat(policyFilePath); os.IsNotExist(err) {
		return false
	}

	return true
}

// IsRequired determines if a maintenance window is required to update the given module.
func (_ MaintenanceWindow) IsRequired(moduleTemplate *v1beta2.ModuleTemplate, kyma *v1beta2.Kyma) bool {
	if !moduleTemplate.Spec.RequiresDowntime {
		return false
	}

	if kyma.Spec.SkipMaintenanceWindows {
		return false
	}

	// module not installed yet => no need for maintenance window
	moduleStatus := kyma.Status.GetModuleStatus(moduleTemplate.Spec.ModuleName)
	if moduleStatus == nil {
		return false
	}

	// module already installed in this version => no need for maintenance window
	installedVersion := moduleStatus.Version
	return installedVersion != moduleTemplate.Spec.Version
}

// IsActive determines if a maintenance window is currently active.
func (mw MaintenanceWindow) IsActive(kyma *v1beta2.Kyma) (bool, error) {
	if mw.MaintenanceWindowPolicy == nil {
		return false, ErrNoMaintenanceWindowPolicyConfigured
	}

	runtime := &resolver.Runtime{
		GlobalAccountID: kyma.GetGlobalAccount(),
		Region:          kyma.GetRegion(),
		PlatformRegion:  kyma.GetPlatformRegion(),
		Plan:            kyma.GetPlan(),
	}

	resolvedWindow, err := mw.MaintenanceWindowPolicy.Resolve(runtime)
	if err != nil {
		return false, err
	}

	now := time.Now()
	if now.After(resolvedWindow.Begin) && now.Before(resolvedWindow.End) {
		return true, nil
	}

	return false, nil
}
