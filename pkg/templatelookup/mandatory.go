package templatelookup

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// GetMandatory returns ModuleTemplates TOs (Transfer Objects) which are marked as mandatory modules.
func GetMandatory(ctx context.Context, kymaClient client.Reader) (ModuleTemplatesByModuleName, error) {
	mandatoryModuleTemplateList := &v1beta2.ModuleTemplateList{}
	labelSelector := k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: shared.EnableLabelValue})

	// Fetch mandatory module templates from the Kyma client
	if err := kymaClient.List(ctx, mandatoryModuleTemplateList, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return nil, fmt.Errorf("could not list mandatory ModuleTemplates: %w", err)
	}

	// Initialize the map to hold the highest versioned template for each module name
	mandatoryModules := make(map[string]*ModuleTemplateInfo)
	comparator := NewModuleTemplateComparator()

	for _, moduleTemplate := range mandatoryModuleTemplateList.Items {
		// Skip deleted modules
		if !moduleTemplate.DeletionTimestamp.IsZero() {
			continue
		}

		moduleName := GetModuleName(&moduleTemplate)

		// Compare with the existing module in the map (if exists) to find the higher version
		if existingModuleTemplateInfo, exists := mandatoryModules[moduleName]; exists {
			updatedModuleTemplate, err := comparator.Compare(&moduleTemplate, existingModuleTemplateInfo.ModuleTemplate)
			if err != nil {
				mandatoryModules[moduleName] = &ModuleTemplateInfo{
					ModuleTemplate: nil,
					Err:            err,
				}
				continue
			}
			mandatoryModules[moduleName] = &ModuleTemplateInfo{
				ModuleTemplate: updatedModuleTemplate,
				Err:            nil,
			}
		} else {
			// If the module is encountered for the first time, simply add it
			mandatoryModules[moduleName] = &ModuleTemplateInfo{
				ModuleTemplate: &moduleTemplate,
				Err:            nil,
			}
		}
	}

	return mandatoryModules, nil
}

// GetModuleName returns the name of the module for a given ModuleTemplate.
func GetModuleName(moduleTemplate *v1beta2.ModuleTemplate) string {
	if moduleTemplate.Spec.ModuleName != "" {
		return moduleTemplate.Spec.ModuleName
	}

	// Handle backward compatibility
	return moduleTemplate.Labels[shared.ModuleName]
}

// GetModuleSemverVersion returns the semver version of a module template.
func GetModuleSemverVersion(moduleTemplate *v1beta2.ModuleTemplate) (*semver.Version, error) {
	if moduleTemplate.Spec.Version != "" {
		version, err := semver.NewVersion(moduleTemplate.Spec.Version)
		if err != nil {
			return nil, fmt.Errorf("could not parse version as a semver: %s: %w",
				moduleTemplate.Spec.Version, err)
		}
		return version, nil
	}

	// Handle backward compatibility for versions stored in annotations
	version, err := semver.NewVersion(moduleTemplate.Annotations[shared.ModuleVersionAnnotation])
	if err != nil {
		return nil, fmt.Errorf("could not parse version as a semver %s: %w",
			moduleTemplate.Annotations[shared.ModuleVersionAnnotation], err)
	}
	return version, nil
}

// ModuleTemplateComparator helps compare two ModuleTemplates by version.
type ModuleTemplateComparator struct{}

// NewModuleTemplateComparator creates a new instance of ModuleTemplateComparator.
func NewModuleTemplateComparator() *ModuleTemplateComparator {
	return &ModuleTemplateComparator{}
}

// Compare compares two module templates and returns the one with the higher version.
func (comparator *ModuleTemplateComparator) Compare(firstModuleTemplate, secondModuleTemplate *v1beta2.ModuleTemplate) (*v1beta2.ModuleTemplate, error) {
	firstVersion, err := GetModuleSemverVersion(firstModuleTemplate)
	if err != nil {
		return nil, err
	}

	secondVersion, err := GetModuleSemverVersion(secondModuleTemplate)
	if err != nil {
		return nil, err
	}

	// Return the module with the higher version
	if firstVersion.GreaterThan(secondVersion) {
		return firstModuleTemplate, nil
	}

	return secondModuleTemplate, nil
}
