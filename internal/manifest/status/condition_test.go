package status_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
)

func TestInitializeStatusConditions_DefaultPolicy_AddsDefaultConditions(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(7)
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore

	status.InitializeStatusConditions(manifest)

	conds := manifest.GetStatus().Conditions
	require.Len(t, conds, 2)

	resources := meta.FindStatusCondition(conds, string(status.ConditionTypeResources))
	require.NotNil(t, resources)
	require.Equal(t, apimetav1.ConditionFalse, resources.Status)
	require.Equal(t, string(status.ConditionReasonResourcesAreAvailable), resources.Reason)
	require.Equal(t, "resources are parsed and ready for use", resources.Message)
	require.Equal(t, manifest.GetGeneration(), resources.ObservedGeneration)

	installation := meta.FindStatusCondition(conds, string(status.ConditionTypeInstallation))
	require.NotNil(t, installation)
	require.Equal(t, apimetav1.ConditionFalse, installation.Status)
	require.Equal(t, string(status.ConditionReasonReady), installation.Reason)
	require.Equal(t, "installation is ready and resources can be used", installation.Message)
	require.Equal(t, manifest.GetGeneration(), installation.ObservedGeneration)

	moduleCR := meta.FindStatusCondition(conds, string(status.ConditionTypeModuleCR))
	require.Nil(t, moduleCR)
}

func TestInitializeStatusConditions_CreateAndDelete_AddsModuleCRCondition(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(3)
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete

	status.InitializeStatusConditions(manifest)

	conds := manifest.GetStatus().Conditions
	require.Len(t, conds, 3)

	moduleCR := meta.FindStatusCondition(conds, string(status.ConditionTypeModuleCR))
	require.NotNil(t, moduleCR)
	require.Equal(t, apimetav1.ConditionFalse, moduleCR.Status)
	require.Equal(t, string(status.ConditionReasonModuleCRInstalled), moduleCR.Reason)
	require.Equal(t, "Module CR is installed and ready for use", moduleCR.Message)
	require.Equal(t, manifest.GetGeneration(), moduleCR.ObservedGeneration)
}

func TestInitializeStatusConditions_DoesNotDuplicateExistingConditions(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(10)
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore

	existingResources := apimetav1.Condition{
		Type:               string(status.ConditionTypeResources),
		Status:             apimetav1.ConditionTrue,
		Reason:             "Custom",
		Message:            "custom message",
		ObservedGeneration: 1,
	}
	existingOther := apimetav1.Condition{
		Type:               "Other",
		Status:             apimetav1.ConditionFalse,
		Reason:             "Other",
		Message:            "other message",
		ObservedGeneration: 1,
	}
	manifest.SetStatus(shared.Status{Conditions: []apimetav1.Condition{existingResources, existingOther}})

	status.InitializeStatusConditions(manifest)

	conds := manifest.GetStatus().Conditions
	require.Len(t, conds, 3)

	resources := meta.FindStatusCondition(conds, string(status.ConditionTypeResources))
	require.NotNil(t, resources)
	require.Equal(t, apimetav1.ConditionTrue, resources.Status)
	require.Equal(t, "Custom", resources.Reason)
	require.Equal(t, "custom message", resources.Message)
	require.Equal(t, int64(1), resources.ObservedGeneration)

	installation := meta.FindStatusCondition(conds, string(status.ConditionTypeInstallation))
	require.NotNil(t, installation)
	require.Equal(t, apimetav1.ConditionFalse, installation.Status)
	require.Equal(t, manifest.GetGeneration(), installation.ObservedGeneration)

	other := meta.FindStatusCondition(conds, "Other")
	require.NotNil(t, other)
}

func TestInitializeStatusConditions_RemovesModuleCRConditionWhenPolicyChanges(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(5)
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore

	moduleCR := apimetav1.Condition{
		Type:               string(status.ConditionTypeModuleCR),
		Status:             apimetav1.ConditionTrue,
		Reason:             string(status.ConditionReasonModuleCRInstalled),
		Message:            "Module CR is installed and ready for use",
		ObservedGeneration: 5,
	}
	manifest.SetStatus(shared.Status{Conditions: []apimetav1.Condition{moduleCR}})

	status.InitializeStatusConditions(manifest)

	conds := manifest.GetStatus().Conditions
	require.Nil(t, meta.FindStatusCondition(conds, string(status.ConditionTypeModuleCR)))
	require.NotNil(t, meta.FindStatusCondition(conds, string(status.ConditionTypeResources)))
	require.NotNil(t, meta.FindStatusCondition(conds, string(status.ConditionTypeInstallation)))
}

func TestIsModuleCRInstallConditionTrue(t *testing.T) {
	t.Run("missing condition", func(t *testing.T) {
		statusValue := shared.Status{Conditions: nil}
		require.False(t, status.IsModuleCRInstallConditionTrue(statusValue))
	})

	t.Run("present but false", func(t *testing.T) {
		statusValue := shared.Status{
			Conditions: []apimetav1.Condition{
				{
					Type:   string(status.ConditionTypeModuleCR),
					Status: apimetav1.ConditionFalse,
				},
			},
		}
		require.False(t, status.IsModuleCRInstallConditionTrue(statusValue))
	})

	t.Run("present and true", func(t *testing.T) {
		statusValue := shared.Status{
			Conditions: []apimetav1.Condition{
				{
					Type:   string(status.ConditionTypeModuleCR),
					Status: apimetav1.ConditionTrue,
				},
			},
		}
		require.True(t, status.IsModuleCRInstallConditionTrue(statusValue))
	})
}

func TestSetModuleCRInstallConditionTrue(t *testing.T) {
	t.Run("condition missing - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete
		status.InitializeStatusConditions(manifest)
		statusObj := manifest.GetStatus()
		statusObj.Conditions = nil
		manifest.SetStatus(statusObj)

		status.SetModuleCRInstallConditionTrue(manifest)

		require.Empty(t, manifest.GetStatus().Conditions)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition already true - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete
		status.InitializeStatusConditions(manifest)
		statusObj := manifest.GetStatus()

		moduleCR := meta.FindStatusCondition(statusObj.Conditions, string(status.ConditionTypeModuleCR))
		require.NotNil(t, moduleCR)
		moduleCR.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&statusObj.Conditions, *moduleCR)
		manifest.SetStatus(statusObj)

		status.SetModuleCRInstallConditionTrue(manifest)

		moduleCR = meta.FindStatusCondition(manifest.GetStatus().Conditions, string(status.ConditionTypeModuleCR))
		require.NotNil(t, moduleCR)
		require.Equal(t, apimetav1.ConditionTrue, moduleCR.Status)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition false - becomes true and sets operation", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete
		status.InitializeStatusConditions(manifest)

		status.SetModuleCRInstallConditionTrue(manifest)

		moduleCR := meta.FindStatusCondition(manifest.GetStatus().Conditions, string(status.ConditionTypeModuleCR))
		require.NotNil(t, moduleCR)
		require.Equal(t, apimetav1.ConditionTrue, moduleCR.Status)
		require.Equal(t, "Module CR is installed and ready for use", manifest.GetStatus().Operation)
	})
}

func TestSetResourcesConditionTrue(t *testing.T) {
	t.Run("condition missing - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		statusObj := manifest.GetStatus()
		statusObj.Conditions = nil
		manifest.SetStatus(statusObj)

		status.SetResourcesConditionTrue(manifest)

		require.Empty(t, manifest.GetStatus().Conditions)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition already true - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
		status.InitializeStatusConditions(manifest)
		statusObj := manifest.GetStatus()

		resources := meta.FindStatusCondition(statusObj.Conditions, string(status.ConditionTypeResources))
		require.NotNil(t, resources)
		resources.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&statusObj.Conditions, *resources)
		manifest.SetStatus(statusObj)

		status.SetResourcesConditionTrue(manifest)

		resources = meta.FindStatusCondition(manifest.GetStatus().Conditions, string(status.ConditionTypeResources))
		require.NotNil(t, resources)
		require.Equal(t, apimetav1.ConditionTrue, resources.Status)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition false - becomes true and sets operation", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
		status.InitializeStatusConditions(manifest)

		status.SetResourcesConditionTrue(manifest)

		resources := meta.FindStatusCondition(manifest.GetStatus().Conditions, string(status.ConditionTypeResources))
		require.NotNil(t, resources)
		require.Equal(t, apimetav1.ConditionTrue, resources.Status)
		require.Equal(t, "resources are parsed and ready for use", manifest.GetStatus().Operation)
	})
}

func TestSetInstallationConditionTrue(t *testing.T) {
	t.Run("condition missing - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		statusObj := manifest.GetStatus()
		statusObj.Conditions = nil
		manifest.SetStatus(statusObj)

		status.SetInstallationConditionTrue(manifest)

		require.Empty(t, manifest.GetStatus().Conditions)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition already true - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
		status.InitializeStatusConditions(manifest)
		statusObj := manifest.GetStatus()

		installation := meta.FindStatusCondition(statusObj.Conditions, string(status.ConditionTypeInstallation))
		require.NotNil(t, installation)
		installation.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&statusObj.Conditions, *installation)
		manifest.SetStatus(statusObj)

		status.SetInstallationConditionTrue(manifest)

		installation = meta.FindStatusCondition(manifest.GetStatus().Conditions,
			string(status.ConditionTypeInstallation))
		require.NotNil(t, installation)
		require.Equal(t, apimetav1.ConditionTrue, installation.Status)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition false - becomes true and sets operation", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
		status.InitializeStatusConditions(manifest)

		status.SetInstallationConditionTrue(manifest)

		installation := meta.FindStatusCondition(manifest.GetStatus().Conditions,
			string(status.ConditionTypeInstallation))
		require.NotNil(t, installation)
		require.Equal(t, apimetav1.ConditionTrue, installation.Status)
		require.Equal(t, "installation is ready and resources can be used", manifest.GetStatus().Operation)
	})
}
