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

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"

	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/lookup"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/module/parse"
	"github.com/kyma-project/lifecycle-manager/pkg/module/sync"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/signature"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type MandatoryModulesReconciler struct {
	client.Client
	record.EventRecorder
	queue.RequeueIntervals
	signature.VerificationSettings
	KcpRestConfig       *rest.Config
	RemoteClientCache   *remote.ClientCache
	ResolveRemoteClient RemoteClientResolver
	RemoteSyncNamespace string
	InKCPMode           bool
}

//nolint:lll
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates/finalizers,verbs=update
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;create;update;delete;patch;watch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions/status,verbs=update

func (r *MandatoryModulesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("reconciling")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if !util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{}, fmt.Errorf("KymaController: %w", err)
		}
		// if indeed not found, stop put this kyma in queue
		return ctrl.Result{Requeue: false}, nil
	}

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info(fmt.Sprintf("skipping mandatory modules reconciliation for Kyma: %s", kyma.Name))
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	}

	// remoteClient, err := r.ResolveRemoteClient(ctx, client.ObjectKeyFromObject(kyma))
	// if util.IsNotFound(err) {
	// 	// TODO check what needs to be done here; Am I going to introduce any finalizers,
	// 	// if yes, they need to be dropped here
	// 	return ctrl.Result{Requeue: true}, nil
	// }
	// if err != nil {
	// 	return ctrl.Result{}, err
	// }

	// TODO:
	// 1. List all Mandatory Modules
	// 2. Filter Moudles by Kyma channel; should take Kyma channel fomr KCP or SKR Kyma? -> needs to be clarified
	// 3. Create Manifest for corresponding mandatory modules

	mandatoryTemplates, err := lookup.GetMandatoryTemplates(ctx, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	modules, err := r.GenerateModulesFromTemplate(ctx, mandatoryTemplates, kyma)
	if err != nil {
		// TODO
	}

	runner := sync.New(r)

	if err := runner.ReconcileManifests(ctx, kyma, modules); err != nil {
		// TODO
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MandatoryModulesReconciler) GenerateModulesFromTemplate(ctx context.Context,
	templates lookup.ModuleTemplatesByModuleName, kyma *v1beta2.Kyma) (common.Modules, error) {

	parser := parse.NewParser(r.Client, r.InKCPMode,
		r.RemoteSyncNamespace, r.EnableVerification, r.PublicKeyFilePath)

	return parser.GenerateMandatoryModulesFromTemplates(ctx, kyma, templates), nil
}
