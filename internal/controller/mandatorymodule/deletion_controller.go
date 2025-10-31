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
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/deletion"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	ctrl "sigs.k8s.io/controller-runtime"
)

type DeletionService interface {
	HandleDeletion(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) error
}

type DeletionReconciler struct {
	deletionService  DeletionService
	requeueIntervals queue.RequeueIntervals
}

func NewDeletionReconciler(deletionService DeletionService,
	requeueIntervals queue.RequeueIntervals,
) *DeletionReconciler {
	return &DeletionReconciler{
		deletionService:  deletionService,
		requeueIntervals: requeueIntervals,
	}
}

func (r *DeletionReconciler) Reconcile(ctx context.Context, mrm *v1beta2.ModuleReleaseMeta) (ctrl.Result, error) {
	err := r.deletionService.HandleDeletion(ctx, mrm)
	return r.determineRequeueBehaviour(err)
}

func (r *DeletionReconciler) determineRequeueBehaviour(err error) (ctrl.Result, error) {
	if err != nil {
		if errors.Is(err, deletion.ErrMrmNotInDeletingState) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("mandatory module deletion reconciliation failed: %w", err)
	}
	return ctrl.Result{RequeueAfter: r.requeueIntervals.Success}, nil
}
