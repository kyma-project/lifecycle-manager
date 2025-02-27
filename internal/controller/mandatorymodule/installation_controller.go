package mandatorymodule

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/internal/service"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type InstallationReconciler struct {
	client.Client
	queue.RequeueIntervals
	KymaService *service.KymaService
}

func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Mandatory Module Reconciliation started")

	kyma, err := r.KymaService.GetKyma(ctx, req.NamespacedName)
	if err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, fmt.Errorf("MandatoryModuleController: %w", err)
	}

	if kyma.SkipReconciliation() {
		logger.V(log.DebugLevel).Info("skipping mandatory modules reconciliation for Kyma: " + kyma.Name)
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, nil
	}

	result, err := r.KymaService.ReconcileMandatoryModules(ctx, kyma)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile mandatory modules: %w", err)
	}

	return result, nil
}
