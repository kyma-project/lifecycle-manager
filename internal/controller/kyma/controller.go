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

package kyma

// This file contains legacy type definitions for backward compatibility.
// The actual reconcilers have been split into:
// - installation_controller.go for installation/update logic
// - deletion_controller.go for deletion logic
//
// All shared utilities and types are now in shared_utils.go

// Reconciler is kept for backward compatibility - it's now an alias to InstallationReconciler
type Reconciler = InstallationReconciler
