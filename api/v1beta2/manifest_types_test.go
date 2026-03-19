/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

func TestShouldCreateDefaultModuleCR(t *testing.T) {
	resource := &unstructured.Unstructured{}

	tests := []struct {
		name     string
		manifest *v1beta2.Manifest
		want     bool
	}{
		{
			name: "returns true when policy is CreateAndDelete, resource is set, and ModuleCR condition is absent",
			manifest: &v1beta2.Manifest{
				Spec: v1beta2.ManifestSpec{
					CustomResourcePolicy: v1beta2.CustomResourcePolicyCreateAndDelete,
					Resource:             resource,
				},
			},
			want: true,
		},
		{
			name: "returns false when policy is Ignore",
			manifest: &v1beta2.Manifest{
				Spec: v1beta2.ManifestSpec{
					CustomResourcePolicy: v1beta2.CustomResourcePolicyIgnore,
					Resource:             resource,
				},
			},
			want: false,
		},
		{
			name: "returns false when resource is nil",
			manifest: &v1beta2.Manifest{
				Spec: v1beta2.ManifestSpec{
					CustomResourcePolicy: v1beta2.CustomResourcePolicyCreateAndDelete,
					Resource:             nil,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.manifest.ShouldCreateDefaultModuleCR()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetCacheKey(t *testing.T) {
	manifest := &v1beta2.Manifest{}
	manifest.SetName("test-manifest")
	manifest.SetNamespace("test-namespace")
	manifest.SetLabels(map[string]string{
		shared.KymaName: "kyma-test",
	})

	expectedKey := "kyma-test|test-namespace"

	key, found := manifest.GenerateCacheKey()
	require.True(t, found)
	require.Equal(t, expectedKey, key)
}
