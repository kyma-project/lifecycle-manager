package manifest_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

func TestHandleState(t *testing.T) {
	t.Parallel()
	type moduleCheck struct {
		fields []string
		value  string
	}
	definedValueForError := "customStateForError"
	definedValueForReady := "customStateForReady"
	tests := []struct {
		name                string
		customState         []*v1beta2.CustomStateCheck
		customStateExpected bool
		checkInModuleCR     []moduleCheck
		want                declarativev2.StateInfo
		wantErr             bool
	}{
		{
			"kyma module with Ready state, expected mapped to StateReady",
			nil,
			false,
			[]moduleCheck{
				{
					[]string{"status", "state"},
					string(shared.StateReady),
				},
			},
			declarativev2.StateInfo{State: shared.StateReady},
			false,
		},
		{
			"kyma module with unsupported State, expected mapped to StateWarning",
			nil,
			false,
			[]moduleCheck{
				{
					[]string{"status", "state"},
					"not support state",
				},
			},
			declarativev2.StateInfo{State: shared.StateWarning, Info: manifest.ErrNotSupportedState.Error()},
			false,
		},
		{
			"custom module with no CustomStateCheckAnnotation, expected mapped to StateProcessing",
			nil,
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customState",
				},
			},
			declarativev2.StateInfo{State: shared.StateProcessing, Info: manifest.ModuleCRWithNoCustomCheckWarning},
			false,
		},
		{
			"custom module with not all required StateCheck, expected mapped to StateError with error",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       "customState",
					MappedState: shared.StateReady,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customState",
				},
			},
			declarativev2.StateInfo{State: shared.StateError},
			true,
		},
		{
			"custom module found mapped value with StateReady, expected mapped to StateReady",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					definedValueForReady,
				},
			},
			declarativev2.StateInfo{State: shared.StateReady},
			false,
		},
		{
			"custom module found mapped value with StateError, expected mapped to StateError",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					definedValueForError,
				},
			},
			declarativev2.StateInfo{State: shared.StateError},
			false,
		},
		{
			"custom module with additional StateCheck, expected mapped to correct state",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       "customStateForWarning",
					MappedState: shared.StateWarning,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customStateWithOtherValue",
				},
				{
					[]string{"fieldLevel3", "fieldLevel4", "fieldLevel5"},
					"customStateForWarning",
				},
			},
			declarativev2.StateInfo{State: shared.StateWarning},
			false,
		},
		{
			"custom module with multiple StateReady condition, expected mapped to StateReady when all condition matched",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					definedValueForReady,
				},
				{
					[]string{"fieldLevel3", "fieldLevel4", "fieldLevel5"},
					definedValueForReady,
				},
			},
			declarativev2.StateInfo{State: shared.StateReady},
			false,
		},
		{
			"custom module with multiple StateReady condition, expected " +
				"mapped to StateProcessing when not all condition matched",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"not in defined value",
				},
				{
					[]string{"fieldLevel3", "fieldLevel4", "fieldLevel5"},
					definedValueForReady,
				},
			},
			declarativev2.StateInfo{State: shared.StateProcessing},
			false,
		},
		{
			"custom module with additional StateCheck but no mapped value found, expected mapped to StateProcessing",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       "customStateForWarning",
					MappedState: shared.StateWarning,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customStateWithOtherValue",
				},
				{
					[]string{"fieldLevel3", "fieldLevel4", "fieldLevel5"},
					"customStateWithOtherValue",
				},
			},
			declarativev2.StateInfo{State: shared.StateProcessing},
			false,
		},
		{
			"custom module not in mapped value, expected mapped to StateProcessing",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customStateWithOtherValue",
				},
			},
			declarativev2.StateInfo{State: shared.StateProcessing},
			false,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			manifestCR := testutils.NewTestManifest("test")
			if testCase.customStateExpected {
				if testCase.customState != nil {
					marshal, err := json.Marshal(testCase.customState)
					if err != nil {
						t.Errorf("HandleState() error = %v", err)
						return
					}
					manifestCR.Annotations[v1beta2.CustomStateCheckAnnotation] = string(marshal)
				}
			}
			manifestCR.CreationTimestamp = apimetav1.Now()
			moduleCR := builder.NewModuleCRBuilder().WithName("test").WithNamespace(apimetav1.NamespaceDefault).
				WithGroupVersionKind(v1beta2.GroupVersion.Group, "v1", "TestCR").Build()
			for _, check := range testCase.checkInModuleCR {
				err := unstructured.SetNestedField(moduleCR.Object, check.value, check.fields...)
				if err != nil {
					t.Errorf("HandleState() error = %v", err)
					return
				}
			}
			got, err := manifest.HandleState(manifestCR, moduleCR)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("HandleState() got = %v, want %v", got, testCase.want)
			}
			if (err != nil) != testCase.wantErr {
				t.Errorf("HandleState() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}

func TestHandleStateWithDuration(t *testing.T) {
	t.Parallel()
	type moduleCheck struct {
		fields []string
		value  string
	}
	definedValueForError := "customStateForError"
	definedValueForReady := "customStateForReady"
	tests := []struct {
		name                string
		customState         []*v1beta2.CustomStateCheck
		customStateExpected bool
		manifestCreatedAt   apimetav1.Time
		checkInModuleCR     []moduleCheck
		want                declarativev2.StateInfo
		wantErr             bool
	}{
		{
			"kyma module just created with no state, expected to StateProcessing",
			nil,
			false,
			apimetav1.Now(),
			nil,
			declarativev2.StateInfo{State: shared.StateProcessing, Info: manifest.ModuleCRWithNoCustomCheckWarning},
			false,
		},
		{
			"kyma module with state updated, expected to StateReady",
			nil,
			false,
			apimetav1.Now(),
			[]moduleCheck{
				{
					[]string{"status", "state"},
					string(shared.StateReady),
				},
			},
			declarativev2.StateInfo{State: shared.StateReady},
			false,
		},
		{
			"kyma module with no state after certain time, expected to StateWarning",
			nil,
			false,
			apimetav1.NewTime(apimetav1.Now().Add(-10 * time.Minute)),
			nil,
			declarativev2.StateInfo{State: shared.StateWarning, Info: manifest.ModuleCRWithNoCustomCheckWarning},
			false,
		},
		{
			"custom module with wrong JSON path after certain time, expected to StateWarning",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: shared.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: shared.StateError,
				},
			},
			true,
			apimetav1.NewTime(apimetav1.Now().Add(-10 * time.Minute)),
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel3"},
					definedValueForReady,
				},
			},
			declarativev2.StateInfo{State: shared.StateWarning, Info: manifest.ModuleCRWithCustomCheckWarning},
			false,
		},
	}
	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			manifestCR := testutils.NewTestManifest("test")
			if testCase.customStateExpected {
				if testCase.customState != nil {
					marshal, err := json.Marshal(testCase.customState)
					if err != nil {
						t.Errorf("HandleState() error = %v", err)
						return
					}
					manifestCR.Annotations[v1beta2.CustomStateCheckAnnotation] = string(marshal)
				}
			}
			manifestCR.CreationTimestamp = testCase.manifestCreatedAt
			moduleCR := builder.NewModuleCRBuilder().WithName("test").WithNamespace(apimetav1.NamespaceDefault).
				WithGroupVersionKind(v1beta2.GroupVersion.Group, "v1", "TestCR").Build()
			for _, check := range testCase.checkInModuleCR {
				err := unstructured.SetNestedField(moduleCR.Object, check.value, check.fields...)
				if err != nil {
					t.Errorf("HandleState() error = %v", err)
					return
				}
			}
			got, err := manifest.HandleState(manifestCR, moduleCR)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("HandleState() got = %v, want %v", got, testCase.want)
			}
			if (err != nil) != testCase.wantErr {
				t.Errorf("HandleState() error = %v, wantErr %v", err, testCase.wantErr)
			}
		})
	}
}
