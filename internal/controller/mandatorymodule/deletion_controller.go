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

package mandatorymodule

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/service"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	settingFinalizerError event.Reason = "SettingMandatoryModuleTemplateFinalizerError"
	deletingManifestError event.Reason = "DeletingMandatoryModuleManifestError"
)

type DeletionReconciler struct {
	event.Event
	queue.RequeueIntervals
	moduleTemplateService          *service.ModuleTemplateService
	mandatoryModuleDeletionService *service.MandatoryModuleDeletionService
}

func NewDeletionReconciler(client client.Client, event event.Event,
	descriptorProvider *provider.CachedDescriptorProvider,
	requeueIntervals queue.RequeueIntervals,
) *DeletionReconciler {
	return &DeletionReconciler{
		Event:                          event,
		RequeueIntervals:               requeueIntervals,
		moduleTemplateService:          service.NewModuleTemplateService(client),
		mandatoryModuleDeletionService: service.NewMandatoryModuleDeletionService(client, descriptorProvider),
	}
}

func (r *DeletionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Mandatory Module Deletion Reconciliation started")

	template, err := r.moduleTemplateService.GetModuleTemplate(ctx, req.NamespacedName)
	if err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("ModuleTemplate %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, fmt.Errorf("MandatoryModuleDeletionController: %w", err)
	}

	if !template.Spec.Mandatory {
		logger.V(log.DebugLevel).Info("Skipping reconciliation, ModuleTemplate not marked as mandatory")
		return ctrl.Result{}, nil
	}

	updateRequired, err := r.moduleTemplateService.UpdateFinalizer(ctx, template, shared.MandatoryModuleFinalizer)
	if err != nil {
		r.Event.Warning(template, settingFinalizerError, err)
		return ctrl.Result{}, fmt.Errorf("failed to update ModuleTemplate finalizer: %w", err)
	}
	if updateRequired {
		return ctrl.Result{Requeue: true}, nil
	}

	if template.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	noManifestLeft, err := r.mandatoryModuleDeletionService.DeleteMandatoryModules(ctx, template)
	if err != nil {
		r.Event.Warning(template, deletingManifestError, err)
		return ctrl.Result{}, fmt.Errorf("failed to delete MandatoryModuleManifests: %w", err)
	}
	if noManifestLeft {
		updateRequired, err := r.moduleTemplateService.RemoveFinalizer(ctx, template, shared.MandatoryModuleFinalizer)
		if err != nil {
			r.Event.Warning(template, settingFinalizerError, err)
			return ctrl.Result{}, fmt.Errorf("failed to update ModuleTemplate finalizer: %w", err)
		}
		if updateRequired {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{Requeue: true}, nil
}
