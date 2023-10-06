package manifest_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

//nolint:funlen,maintidx
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
		want                v2.StateInfo
		wantErr             bool
	}{
		{
			"kyma module with Ready state, expected mapped to StateReady",
			nil,
			false,
			[]moduleCheck{
				{
					[]string{"status", "state"},
					string(v2.StateReady),
				},
			},
			v2.StateInfo{State: v2.StateReady},
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
			v2.StateInfo{State: v2.StateWarning, Info: manifest.ErrNotSupportedState.Error()},
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
			v2.StateInfo{State: v2.StateProcessing, Info: manifest.ModuleCRWithNoCustomCheckWarning},
			false,
		},
		{
			"custom module with not all required StateCheck, expected mapped to StateError with error",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       "customState",
					MappedState: v1beta2.StateReady,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customState",
				},
			},
			v2.StateInfo{State: v2.StateError},
			true,
		},
		{
			"custom module found mapped value with StateReady, expected mapped to StateReady",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					definedValueForReady,
				},
			},
			v2.StateInfo{State: v2.StateReady},
			false,
		},
		{
			"custom module found mapped value with StateError, expected mapped to StateError with error",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					definedValueForError,
				},
			},
			v2.StateInfo{State: v2.StateError},
			true,
		},
		{
			"custom module with additional StateCheck, expected mapped to correct state",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       "customStateForWarning",
					MappedState: v1beta2.StateWarning,
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
			v2.StateInfo{State: v2.StateWarning},
			false,
		},
		{
			"custom module with multiple StateReady condition, expected mapped to StateReady when all condition matched",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
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
			v2.StateInfo{State: v2.StateReady},
			false,
		},
		{
			"custom module with multiple StateReady condition, expected mapped to StateProcessing when not all condition matched", //nolint:lll
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
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
			v2.StateInfo{State: v2.StateProcessing},
			false,
		},
		{
			"custom module with additional StateCheck but no mapped value found, expected mapped to StateProcessing",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
				{
					JSONPath:    "fieldLevel3.fieldLevel4.fieldLevel5",
					Value:       "customStateForWarning",
					MappedState: v1beta2.StateWarning,
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
			v2.StateInfo{State: v2.StateProcessing},
			false,
		},
		{
			"custom module not in mapped value, expected mapped to StateProcessing",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
			},
			true,
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel2"},
					"customStateWithOtherValue",
				},
			},
			v2.StateInfo{State: v2.StateProcessing},
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
			manifestCR.CreationTimestamp = v1.Now()
			moduleCR := builder.NewModuleCRBuilder().WithName("test").WithNamespace(v1.NamespaceDefault).
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

//nolint:funlen
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
		manifestCreatedAt   v1.Time
		checkInModuleCR     []moduleCheck
		want                v2.StateInfo
		wantErr             bool
	}{
		{
			"kyma module just created with no state, expected to StateProcessing",
			nil,
			false,
			v1.Now(),
			nil,
			v2.StateInfo{State: v2.StateProcessing, Info: manifest.ModuleCRWithNoCustomCheckWarning},
			false,
		},
		{
			"kyma module with state updated, expected to StateReady",
			nil,
			false,
			v1.Now(),
			[]moduleCheck{
				{
					[]string{"status", "state"},
					string(v2.StateReady),
				},
			},
			v2.StateInfo{State: v2.StateReady},
			false,
		},
		{
			"kyma module with no state after certain time, expected to StateWarning",
			nil,
			false,
			v1.NewTime(v1.Now().Add(-10 * time.Minute)),
			nil,
			v2.StateInfo{State: v2.StateWarning, Info: manifest.ModuleCRWithNoCustomCheckWarning},
			false,
		},
		{
			"custom module with wrong JSON path after certain time, expected to StateWarning",
			[]*v1beta2.CustomStateCheck{
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForReady,
					MappedState: v1beta2.StateReady,
				},
				{
					JSONPath:    "fieldLevel1.fieldLevel2",
					Value:       definedValueForError,
					MappedState: v1beta2.StateError,
				},
			},
			true,
			v1.NewTime(v1.Now().Add(-10 * time.Minute)),
			[]moduleCheck{
				{
					[]string{"fieldLevel1", "fieldLevel3"},
					definedValueForReady,
				},
			},
			v2.StateInfo{State: v2.StateWarning, Info: manifest.ModuleCRWithCustomCheckWarning},
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
			moduleCR := builder.NewModuleCRBuilder().WithName("test").WithNamespace(v1.NamespaceDefault).
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
