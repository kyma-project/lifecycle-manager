package fromerror_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/service/kyma/status/modules/generator/fromerror"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/common"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup/moduletemplateinfolookup"
)

func TestGenerateModuleStatusFromError_WhenCalledWithAnyOtherError_ReturnsDefaultNewStatusWithStateError(t *testing.T) {
	someModuleName := "some-module"
	someChannel := "some-channel"
	someFQDN := "some-fqdn"
	status := createStatus()
	templateError := errors.New("some error")

	result, err := fromerror.GenerateModuleStatusFromError(templateError, someModuleName, someChannel, someFQDN, status)

	assert.NotNil(t, result)
	require.NoError(t, err)
	require.NotEqual(t, status.DeepCopy(), result)

	// Module info is used for creating a new status
	assert.Equal(t, someModuleName, result.Name)
	assert.Equal(t, someChannel, result.Channel)
	assert.Equal(t, someFQDN, result.FQDN)

	assert.Equal(t, shared.StateError, result.State)
	assert.Equal(t, templateError.Error(), result.Message)

	// No tracking objects are set
	assert.Nil(t, result.Manifest)
	assert.Nil(t, result.Resource)
	assert.Nil(t, result.Template)
}

func TestGenerateModuleStatusFromError_WhenCalledWithMaintenanceWindowActiveError_ReturnsDeepCopyAndMessage(
	t *testing.T,
) {
	someModuleName := "some-module"
	someChannel := "some-channel"
	someFQDN := "some-fqdn"
	status := createStatus()
	templateError := moduletemplateinfolookup.ErrWaitingForNextMaintenanceWindow

	result, err := fromerror.GenerateModuleStatusFromError(templateError, someModuleName, someChannel, someFQDN, status)

	assert.NotNil(t, result)
	require.NoError(t, err)

	expectedStatus := status.DeepCopy()
	expectedStatus.Message = templateError.Error()
	expectedStatus.Maintenance = true
	assert.Equal(t, expectedStatus, result)

	// Passed module info is not used for new status, but the deep-copied object
	assert.NotEqual(t, someModuleName, result.Name)
	assert.NotEqual(t, someChannel, result.Channel)
	assert.NotEqual(t, someFQDN, result.FQDN)
}

func TestGenerateModuleStatusFromError_WhenCalledWithMaintenanceWindowUnknownError_ReturnsDeepCopyAndStateError(
	t *testing.T,
) {
	someModuleName := "some-module"
	someChannel := "some-channel"
	someFQDN := "some-fqdn"
	status := createStatus()
	templateError := moduletemplateinfolookup.ErrFailedToDetermineIfMaintenanceWindowIsActive

	result, err := fromerror.GenerateModuleStatusFromError(templateError, someModuleName, someChannel, someFQDN, status)

	assert.NotNil(t, result)
	require.NoError(t, err)

	expectedStatus := status.DeepCopy()
	expectedStatus.Message = templateError.Error()
	expectedStatus.State = shared.StateError
	assert.Equal(t, expectedStatus, result)

	// Passed module info is not used for new status, but the deep-copied object
	assert.NotEqual(t, someModuleName, result.Name)
	assert.NotEqual(t, someChannel, result.Channel)
	assert.NotEqual(t, someFQDN, result.FQDN)
}

func TestGenerateModuleStatusFromError_WhenCalledWithTemplateUpdateNotAllowedError_ReturnsDeepCopyAndStateWarning(
	t *testing.T,
) {
	someModuleName := "some-module"
	someChannel := "some-channel"
	someFQDN := "some-fqdn"
	status := createStatus()
	templateError := templatelookup.ErrTemplateUpdateNotAllowed

	result, err := fromerror.GenerateModuleStatusFromError(templateError, someModuleName, someChannel, someFQDN, status)

	assert.NotNil(t, result)
	require.NoError(t, err)

	expectedStatus := status.DeepCopy()
	expectedStatus.Message = templateError.Error()
	expectedStatus.State = shared.StateWarning
	assert.Equal(t, expectedStatus, result)

	// Passed module info is not used for new status, but the deep-copied object
	assert.NotEqual(t, someModuleName, result.Name)
	assert.NotEqual(t, someChannel, result.Channel)
	assert.NotEqual(t, someFQDN, result.FQDN)
}

func TestGenerateModuleStatusFromError_WhenCalledWithNoTemplatesInListResultError_ReturnsNewStatusWithStateWarning(
	t *testing.T,
) {
	expectedModuleName := "some-module"
	expectedChannel := "some-channel"
	expectedFQDN := "some-fqdn"
	status := createStatus()
	templateError := common.ErrNoTemplatesInListResult

	result, err := fromerror.GenerateModuleStatusFromError(templateError, expectedModuleName, expectedChannel,
		expectedFQDN, status)

	assert.NotNil(t, result)
	require.NoError(t, err)
	assert.NotEqual(t, status.DeepCopy(), result)

	// Module info is used for creating a new status
	assert.Equal(t, expectedModuleName, result.Name)
	assert.Equal(t, expectedChannel, result.Channel)
	assert.Equal(t, expectedFQDN, result.FQDN)

	assert.Equal(t, shared.StateWarning, result.State)
	assert.Equal(t, templateError.Error(), result.Message)

	// No tracking objects are set
	assert.Nil(t, result.Manifest)
	assert.Nil(t, result.Resource)
	assert.Nil(t, result.Template)
}

func TestGenerateModuleStatusFromError_WhenCalledWithoutTemplateError_ReturnsErr(t *testing.T) {
	_, err := fromerror.GenerateModuleStatusFromError(nil, "", "", "", &v1beta2.ModuleStatus{})
	require.Error(t, err)
}

func TestGenerateModuleStatusFromError_WhenCalledWithNilStatus_ReturnsNewDefaultModuleStatus(t *testing.T) {
	expectedName := "some-module"
	expectedChannel := "some-channel"
	expectedFQDN := "some-fqdn"
	templateError := errors.New("some-error")

	result, err := fromerror.GenerateModuleStatusFromError(templateError, expectedName, expectedChannel, expectedFQDN,
		nil)

	assert.NotNil(t, result)
	require.NoError(t, err)
	assert.Equal(t, expectedName, result.Name)
	assert.Equal(t, expectedChannel, result.Channel)
	assert.Equal(t, expectedFQDN, result.FQDN)
}

// Resource creator helper functions

func createStatus() *v1beta2.ModuleStatus {
	return &v1beta2.ModuleStatus{
		Name:     "test-module",
		Channel:  "test-channel",
		FQDN:     "test-fqdn",
		Version:  "test-version",
		Message:  "test-message",
		State:    shared.StateReady,
		Manifest: createTrackingObject(),
		Template: createTrackingObject(),
		Resource: createTrackingObject(),
	}
}

func createTrackingObject() *v1beta2.TrackingObject {
	return &v1beta2.TrackingObject{
		PartialMeta: v1beta2.PartialMeta{
			Name:       "test-name",
			Namespace:  "test-namespace",
			Generation: 1,
		},
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "test-kind",
			APIVersion: "test-api-version",
		},
	}
}
