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

// Package v1beta1 contains API Schema definitions for the operator v1beta1 API group
// +kubebuilder:object:generate=true
// +groupName=operator.kyma-project.io
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "operator.kyma-project.io", Version: "v1beta1"} //nolint:gochecknoglobals

	// GroupVersionResource is group version resource.
	GroupVersionResource = GroupVersion.WithResource(shared.KymaKind.Plural()) //nolint:gochecknoglobals
)
