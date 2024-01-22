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

package controller

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type MandatoryModuleReconciler struct {
	client.Client
	record.EventRecorder
	queue.RequeueIntervals
	signature.VerificationSettings
	RemoteSyncNamespace string
	InKCPMode           bool
}

func (r *MandatoryModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Mandatory Module Reconciliation started")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, fmt.Errorf("MandatoryModuleController: %w", err)
	}

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info(fmt.Sprintf("skipping mandatory modules reconciliation for Kyma: %s", kyma.Name))
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	}

	mandatoryTemplates, err := templatelookup.GetMandatory(ctx, r.Client)
	if err != nil {
		return emptyResultWithErr(err)
	}

	modules, err := r.GenerateModulesFromTemplate(ctx, mandatoryTemplates, kyma)
	if err != nil {
		return emptyResultWithErr(err)
	}

	runner := sync.New(r)
	if err := runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		return emptyResultWithErr(err)
	}

	return ctrl.Result{}, nil
}

func (r *MandatoryModuleReconciler) GenerateModulesFromTemplate(ctx context.Context,
	templates templatelookup.ModuleTemplatesByModuleName, kyma *v1beta2.Kyma,
) (common.Modules, error) {
	parser := parse.NewParser(r.Client, r.InKCPMode,
		r.RemoteSyncNamespace, r.EnableVerification, r.PublicKeyFilePath)

	return parser.GenerateMandatoryModulesFromTemplates(ctx, kyma, templates), nil
}

func emptyResultWithErr(err error) (ctrl.Result, error) {
	return ctrl.Result{}, fmt.Errorf("MandatoryModuleController: %w", err)
}
