package status_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
)

func TestInitializeStatusConditions_IgnorePolicy_AddsExpectedDefaultConditions(t *testing.T) {
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
	require.Equal(t, "resources not parsed", resources.Message)
	require.Equal(t, manifest.GetGeneration(), resources.ObservedGeneration)

	installation := meta.FindStatusCondition(conds, string(status.ConditionTypeInstallation))
	require.NotNil(t, installation)
	require.Equal(t, apimetav1.ConditionFalse, installation.Status)
	require.Equal(t, string(status.ConditionReasonReady), installation.Reason)
	require.Equal(t, "installation is not ready", installation.Message)
	require.Equal(t, manifest.GetGeneration(), installation.ObservedGeneration)

	moduleCR := meta.FindStatusCondition(conds, string(status.ConditionTypeModuleCR))
	require.Nil(t, moduleCR)
}

func TestInitializeStatusConditions_CreateAndDelete_AddsModuleCRCondition(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(3)
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete
	moduleCRResource := &unstructured.Unstructured{}
	moduleCRResource.SetName("test-resource")
	manifest.Spec.Resource = moduleCRResource

	status.InitializeStatusConditions(manifest)

	conds := manifest.GetStatus().Conditions
	require.Len(t, conds, 3)

	moduleCR := meta.FindStatusCondition(conds, string(status.ConditionTypeModuleCR))
	require.NotNil(t, moduleCR)
	require.Equal(t, apimetav1.ConditionFalse, moduleCR.Status)
	require.Equal(t, string(status.ConditionReasonModuleCRCreated), moduleCR.Reason)
	require.Equal(t, "module CR has not been created to SKR", moduleCR.Message)
	require.Equal(t, manifest.GetGeneration(), moduleCR.ObservedGeneration)
}

func TestInitializeStatusConditions_CreateAndDelete_NoResourceSet_DoesNotAddModuleCRCondition(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetGeneration(3)
	manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyCreateAndDelete

	status.InitializeStatusConditions(manifest)

	conds := manifest.GetStatus().Conditions
	require.Len(t, conds, 2)
	require.Nil(t, meta.FindStatusCondition(conds, string(status.ConditionTypeModuleCR)))
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
		manifest.Spec.Resource = &unstructured.Unstructured{}
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
		manifest.Spec.Resource = &unstructured.Unstructured{}
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
		manifest.Spec.Resource = &unstructured.Unstructured{}
		status.InitializeStatusConditions(manifest)

		status.SetModuleCRInstallConditionTrue(manifest)

		moduleCR := meta.FindStatusCondition(manifest.GetStatus().Conditions, string(status.ConditionTypeModuleCR))
		require.NotNil(t, moduleCR)
		require.Equal(t, apimetav1.ConditionTrue, moduleCR.Status)
		require.Equal(t, manifest.GetGeneration(), moduleCR.ObservedGeneration)
		require.Equal(t, "module CR was created", manifest.GetStatus().Operation)
	})
}

func TestSetResourcesConditionTrue(t *testing.T) {
	testSetConditionTrue(t,
		status.ConditionTypeResources,
		status.SetResourcesConditionTrue,
		"resources are parsed and ready for use",
	)
}

func TestSetInstallationConditionTrue(t *testing.T) {
	testSetConditionTrue(t,
		status.ConditionTypeInstallation,
		status.SetInstallationConditionTrue,
		"installation is ready and resources can be used",
	)
}

func testSetConditionTrue(
	t *testing.T,
	conditionType status.ConditionType,
	setConditionTrue func(*v1beta2.Manifest),
	expectedOperation string,
) {
	t.Helper()

	t.Run("condition missing - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		statusObj := manifest.GetStatus()
		statusObj.Conditions = nil
		manifest.SetStatus(statusObj)

		setConditionTrue(manifest)

		require.Empty(t, manifest.GetStatus().Conditions)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition already true - no-op", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
		status.InitializeStatusConditions(manifest)
		statusObj := manifest.GetStatus()

		cond := meta.FindStatusCondition(statusObj.Conditions, string(conditionType))
		require.NotNil(t, cond)
		cond.Status = apimetav1.ConditionTrue
		meta.SetStatusCondition(&statusObj.Conditions, *cond)
		manifest.SetStatus(statusObj)

		setConditionTrue(manifest)

		cond = meta.FindStatusCondition(manifest.GetStatus().Conditions, string(conditionType))
		require.NotNil(t, cond)
		require.Equal(t, apimetav1.ConditionTrue, cond.Status)
		require.Empty(t, manifest.GetStatus().Operation)
	})

	t.Run("condition false - becomes true and sets operation", func(t *testing.T) {
		manifest := &v1beta2.Manifest{}
		manifest.SetGeneration(1)
		manifest.Spec.CustomResourcePolicy = v1beta2.CustomResourcePolicyIgnore
		status.InitializeStatusConditions(manifest)

		setConditionTrue(manifest)

		cond := meta.FindStatusCondition(manifest.GetStatus().Conditions, string(conditionType))
		require.NotNil(t, cond)
		require.Equal(t, apimetav1.ConditionTrue, cond.Status)
		require.Equal(t, expectedOperation, manifest.GetStatus().Operation)
	})
}
