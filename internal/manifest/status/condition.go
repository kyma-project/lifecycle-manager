package status

import (
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	ConditionReasonModuleCRWarning       ConditionReason = "Warning"
	ConditionReasonReady                 ConditionReason = "Ready"
)

func initInstallationCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeInstallation),
		Reason:             string(ConditionReasonReady),
		Status:             apimetav1.ConditionFalse,
		Message:            "installation is ready and resources can be used",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func initResourcesCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeResources),
		Reason:             string(ConditionReasonResourcesAreAvailable),
		Status:             apimetav1.ConditionFalse,
		Message:            "resources are parsed and ready for use",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func initModuleCRCondition(manifest *v1beta2.Manifest) apimetav1.Condition {
	return apimetav1.Condition{
		Type:               string(ConditionTypeModuleCR),
		Reason:             string(ConditionReasonModuleCRWarning),
		Status:             apimetav1.ConditionFalse,
		Message:            "Module CR is in Warning state",
		ObservedGeneration: manifest.GetGeneration(),
	}
}

func UpdateResourcesCondition(manifest *v1beta2.Manifest) {
	status := manifest.GetStatus()
	resourceCondition := initResourcesCondition(manifest)

	if !meta.IsStatusConditionTrue(status.Conditions, resourceCondition.Type) {
		resourceCondition.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&status.Conditions, resourceCondition)
		manifest.SetStatus(status.WithOperation(resourceCondition.Message))
	}
}
