package status_test

import (
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/status"
)

func TestRequireManifestStateUpdateAfterSyncResource(t *testing.T) {
	tests := []struct {
		name          string
		newState      shared.State
		expectedState shared.State
		expectedOp    string
		expectUpdate  bool
	}{
		{
			name:          "State changes to Processing",
			newState:      shared.StateProcessing,
			expectedState: shared.StateProcessing,
			expectedOp:    status.WaitingForResourcesMsg,
			expectUpdate:  true,
		},
		{
			name:          "State changes to Error",
			newState:      shared.StateError,
			expectedState: shared.StateError,
			expectedOp:    status.WaitingForResourcesMsg,
			expectUpdate:  true,
		},
		{
			name:          "State changes to Ready",
			newState:      shared.StateReady,
			expectedState: shared.StateReady,
			expectedOp:    status.ResourcesAreReadyMsg,
			expectUpdate:  false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			manifest := &v1beta2.Manifest{}
			manifest.SetGeneration(1)
			manifestStatus := shared.Status{
				State: shared.StateReady,
			}
			manifestStatus = manifestStatus.WithOperation(status.ResourcesAreReadyMsg)
			manifest.SetStatus(manifestStatus)

			updated := status.RequireManifestStateUpdateAfterSyncResource(manifest, testCase.newState)
			if updated != testCase.expectUpdate {
				t.Errorf("expected update to be %v, got %v", testCase.expectUpdate, updated)
			}

			if manifest.GetStatus().State != testCase.expectedState {
				t.Errorf("expected state to be %v, got %v", testCase.expectedState, manifest.GetStatus().State)
			}

			if manifest.GetStatus().Operation != testCase.expectedOp {
				t.Errorf("expected operation to be %v, got %v", testCase.expectedOp, manifest.GetStatus().Operation)
			}
		})
	}
}
