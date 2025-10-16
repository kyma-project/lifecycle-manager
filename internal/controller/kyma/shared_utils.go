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

import (
	"context"
	"errors"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/watcher"
)

var (
	ErrManifestsStillExist = errors.New("manifests still exist")
	ErrInvalidKymaSpec     = errors.New("invalid kyma spec")
	ErrKymaInErrorState    = errors.New("kyma in error state")
)

const (
	metricsError      event.Reason = "MetricsError"
	updateSpecError   event.Reason = "UpdateSpecError"
	updateStatusError event.Reason = "UpdateStatusError"
	patchStatusError  event.Reason = "PatchStatus"
)

type SKRWebhookManager interface {
	Reconcile(ctx context.Context, kyma *v1beta2.Kyma) error
	Remove(ctx context.Context, kyma *v1beta2.Kyma) error
	RemoveSkrCertificate(ctx context.Context, kymaName string) error
}

type ModuleStatusHandler interface {
	UpdateModuleStatuses(ctx context.Context, kyma *v1beta2.Kyma, modules modulecommon.Modules) error
}

// checkSKRWebhookReadiness is a shared function used by both controllers
func checkSKRWebhookReadiness(ctx context.Context, skrClient *remote.SkrContext, kyma *v1beta2.Kyma) error {
	err := watcher.AssertDeploymentReady(ctx, skrClient)
	if err != nil {
		kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionFalse)
		if errors.Is(err, watcher.ErrSkrWebhookDeploymentInBackoff) {
			return err
		}
		return nil
	}
	kyma.UpdateCondition(v1beta2.ConditionTypeSKRWebhook, apimetav1.ConditionTrue)
	return nil
}

// needUpdateForMandatoryModuleLabel checks if a module template needs updating for mandatory module labels
func needUpdateForMandatoryModuleLabel(moduleTemplate v1beta2.ModuleTemplate) bool {
	if moduleTemplate.Labels == nil {
		moduleTemplate.Labels = make(map[string]string)
	}

	if moduleTemplate.Spec.Mandatory {
		if moduleTemplate.Labels[shared.IsMandatoryModule] == shared.EnableLabelValue {
			return false
		}

		moduleTemplate.Labels[shared.IsMandatoryModule] = shared.EnableLabelValue
		return true
	}

	if !moduleTemplate.Spec.Mandatory {
		if moduleTemplate.Labels[shared.IsMandatoryModule] == shared.EnableLabelValue {
			delete(moduleTemplate.Labels, shared.IsMandatoryModule)
			return true
		}
	}

	return false
}

// setModuleStatusesToError sets all module statuses to error state
func setModuleStatusesToError(kyma *v1beta2.Kyma, message string) {
	moduleStatuses := kyma.Status.Modules
	for i := range moduleStatuses {
		moduleStatuses[i].State = shared.StateError
		if message != "" {
			moduleStatuses[i].Message = message
		}
	}
}
