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
	"github.com/kyma-project/lifecycle-manager/internal/errors/mandatorymodule/installation"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	ctrl "sigs.k8s.io/controller-runtime"
)

type InstallationService interface {
	HandleInstallation(ctx context.Context, kyma *v1beta2.Kyma) error
}

type InstallationReconciler struct {
	requeueIntervals    queue.RequeueIntervals
	installationService InstallationService
}

func NewInstallationReconciler(requeueIntervals queue.RequeueIntervals,
	installationService InstallationService,
) *InstallationReconciler {
	return &InstallationReconciler{
		requeueIntervals:    requeueIntervals,
		installationService: installationService,
	}
}

func (r *InstallationReconciler) Reconcile(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	err := r.installationService.HandleInstallation(ctx, kyma)
	return r.determineRequeueBehaviour(err)
}

func (r *InstallationReconciler) determineRequeueBehaviour(err error) (ctrl.Result, error) {
	if err != nil {
		if errors.Is(err, installation.ErrSkippingReconciliationKyma) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("mandatory module installation reconciliation failed: %w", err)
	}
	return ctrl.Result{RequeueAfter: r.requeueIntervals.Success}, nil
}
