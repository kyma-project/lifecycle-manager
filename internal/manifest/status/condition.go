package status

import (
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

type ConditionType string

const (
	ConditionTypeResources    ConditionType = "Resources"
	ConditionTypeModuleCR     ConditionType = "ModuleCR"
	ConditionTypeInstallation ConditionType = "Installation"
)

type ConditionReason string

const (
	ConditionReasonResourcesAreAvailable ConditionReason = "ResourcesAvailable"
	ConditionReasonModuleCRInstalled     ConditionReason = "ModuleCRInstalled"
	ConditionReasonReady                 ConditionReason = "Ready"
)

func InitializeStatusConditions(manifest *v1beta2.Manifest) {
	status := manifest.GetStatus()

	for _, condition := range getDefaultConditions(manifest) {
		if meta.FindStatusCondition(status.Conditions, condition.Type) == nil {
			meta.SetStatusCondition(&status.Conditions, condition)
		}
	}

	manifest.SetStatus(status)
}

func getDefaultConditions(manifest *v1beta2.Manifest) []apimetav1.Condition {
	defaultConditions := []apimetav1.Condition{
		getDefaultResourcesCondition(manifest),
		getDefaultInstallationCondition(manifest),
	}
	if manifest.DeploysModuleCR() {
		defaultConditions = append(defaultConditions, getDefaultModuleCRInstalledCondition(manifest))
	}
	return defaultConditions
}

func getDefaultInstallationCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeInstallation),
		Reason:             string(ConditionReasonReady),
		Status:             apimetav1.ConditionFalse,
		Message:            "installation is not ready",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func getDefaultResourcesCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeResources),
		Reason:             string(ConditionReasonResourcesAreAvailable),
		Status:             apimetav1.ConditionFalse,
		Message:            "resources not parsed",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func getDefaultModuleCRInstalledCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeModuleCR),
		Reason:             string(ConditionReasonModuleCRInstalled),
		Status:             apimetav1.ConditionFalse,
		Message:            "module CR has not been deployed to SKR",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func IsModuleCRInstallConditionTrue(status shared.Status) bool {
	condition := meta.FindStatusCondition(status.Conditions, string(ConditionTypeModuleCR))

	return condition != nil && condition.Status == apimetav1.ConditionTrue
}

func SetResourcesConditionTrue(manifest *v1beta2.Manifest) {
	setConditionToTrue(manifest, ConditionTypeResources, "resources are parsed and ready for use")
}

func SetInstallationConditionTrue(manifest *v1beta2.Manifest) {
	setConditionToTrue(manifest, ConditionTypeInstallation, "installation is ready and resources can be used")
}

func SetModuleCRInstallConditionTrue(manifest *v1beta2.Manifest) {
	setConditionToTrue(manifest, ConditionTypeModuleCR, "module CR was deployed")
}

func setConditionToTrue(manifest *v1beta2.Manifest, conditionType ConditionType, message string) {
	status := manifest.GetStatus()
	condition := meta.FindStatusCondition(status.Conditions, string(conditionType))

	if condition != nil && condition.Status != apimetav1.ConditionTrue {
		condition.Status = apimetav1.ConditionTrue
		condition.ObservedGeneration = manifest.GetGeneration()
		condition.Message = message
		meta.SetStatusCondition(&status.Conditions, *condition)
		manifest.SetStatus(status.WithOperation(condition.Message))
	}
}
