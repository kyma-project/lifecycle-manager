package api_test

import (
	"reflect"
	"testing"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestAPISpec(t *testing.T) {
	testCases := []struct {
		name              string
		v1beta1           reflect.Value
		v1beta2           reflect.Value
		v1beta1Exceptions map[string]bool
		v1beta2Exceptions map[string]bool
	}{
		{
			name:              "Kyma",
			v1beta1:           reflect.ValueOf(v1beta1.Kyma{}),
			v1beta2:           reflect.ValueOf(v1beta2.Kyma{}),
			v1beta1Exceptions: map[string]bool{},
			v1beta2Exceptions: map[string]bool{},
		},
		{
			name:              "KymaSpec",
			v1beta1:           reflect.ValueOf(v1beta1.KymaSpec{}),
			v1beta2:           reflect.ValueOf(v1beta2.KymaSpec{}),
			v1beta1Exceptions: map[string]bool{},
			v1beta2Exceptions: map[string]bool{
				"Sync": true,
			},
		},
		{
			name:              "KymaStatus",
			v1beta1:           reflect.ValueOf(v1beta1.KymaStatus{}),
			v1beta2:           reflect.ValueOf(v1beta2.KymaStatus{}),
			v1beta1Exceptions: map[string]bool{},
			v1beta2Exceptions: map[string]bool{},
		},
		{
			name:              "Module",
			v1beta1:           reflect.ValueOf(v1beta1.Module{}),
			v1beta2:           reflect.ValueOf(v1beta2.Module{}),
			v1beta1Exceptions: map[string]bool{},
			v1beta2Exceptions: map[string]bool{},
		},
	}

	for i := range testCases {
		i := i
		t.Run(testCases[i].name, func(t *testing.T) {
			v1beta1 := testCases[i].v1beta1
			v1beta2 := testCases[i].v1beta2
			v1beta1Exceptions := testCases[i].v1beta1Exceptions
			v1beta2Exceptions := testCases[i].v1beta2Exceptions

			fieldNames := map[string]bool{}
			for i := 0; i < v1beta1.NumField(); i++ {
				fieldNames[v1beta1.Type().Field(i).Name] = true
			}
			for i := 0; i < v1beta2.NumField(); i++ {
				fieldNames[v1beta2.Type().Field(i).Name] = true
			}

			for fieldName := range fieldNames {
				v1beta1Field := v1beta1.FieldByName(fieldName)
				v1beta2Field := v1beta2.FieldByName(fieldName)

				if !v1beta1Field.IsValid() {
					if _, exceptionExists := v1beta1Exceptions[fieldName]; exceptionExists {
						continue
					}

					t.Errorf("v1beta1 does not have field %s", fieldName)
					return
				}
				if !v1beta2Field.IsValid() {
					if _, exceptionExists := v1beta2Exceptions[fieldName]; exceptionExists {
						continue
					}

					t.Errorf("v1beta2 does not have field %s", fieldName)
					return
				}

				if v1beta1Field.Type().Name() != v1beta2Field.Type().Name() {
					t.Errorf("%s has different types. In v1beta1: %s, in v1beta2: %s", fieldName, v1beta1Field.Type(), v1beta2Field.Type())
					return
				}
			}

			t.Logf("v1beta1 and v1beta2 have the same fields")
		})
	}
}
