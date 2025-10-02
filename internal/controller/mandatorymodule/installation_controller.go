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

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/templatelookup"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	modulecommon "github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type InstallationReconciler struct {
	client.Client
	queue.RequeueIntervals

	DescriptorProvider  *provider.CachedDescriptorProvider
	RemoteSyncNamespace string
	Metrics             *metrics.MandatoryModulesMetrics
}

func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Mandatory Module Reconciliation started")

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("MandatoryModuleController: %w", err)
	}

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("skipping mandatory modules reconciliation for Kyma: " + kyma.Name)
		return ctrl.Result{RequeueAfter: r.Success}, nil
	}

	mandatoryTemplates, err := templatelookup.GetMandatory(ctx, r.Client)
	if err != nil {
		return emptyResultWithErr(err)
	}
	r.Metrics.RecordMandatoryTemplatesCount(len(mandatoryTemplates))

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

func (r *InstallationReconciler) GenerateModulesFromTemplate(ctx context.Context,
	templates templatelookup.ModuleTemplatesByModuleName, kyma *v1beta2.Kyma,
) (modulecommon.Modules, error) {
	parser := parser.NewParser(r.Client, r.DescriptorProvider, r.RemoteSyncNamespace)
	return parser.GenerateMandatoryModulesFromTemplates(ctx, kyma, templates), nil
}

func emptyResultWithErr(err error) (ctrl.Result, error) {
	return ctrl.Result{}, fmt.Errorf("MandatoryModuleController: %w", err)
}
