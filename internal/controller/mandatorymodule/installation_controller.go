package mandatorymodule

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/parser"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/service"
	"github.com/kyma-project/lifecycle-manager/internal/service/mandatorymodule"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type InstallationReconciler struct {
	client.Client
	queue.RequeueIntervals
	kymaService            *service.KymaService
	mandatoryModuleService *mandatorymodule.MandatoryModuleInstallationService
}

func NewInstallationReconciler(client client.Client,
	requeueIntervals queue.RequeueIntervals,
	parser *parser.Parser,
	metrics *metrics.MandatoryModulesMetrics,
) *InstallationReconciler {
	return &InstallationReconciler{
		Client:                 client,
		RequeueIntervals:       requeueIntervals,
		kymaService:            service.NewKymaService(client),
		mandatoryModuleService: mandatorymodule.NewMandatoryModuleInstallationService(client, metrics, parser),
	}
}

func (r *InstallationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Mandatory Module Reconciliation started")

	kyma, err := r.kymaService.GetKyma(ctx, req.NamespacedName)
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

	if err := r.mandatoryModuleService.ReconcileMandatoryModules(ctx, kyma); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile mandatory modules: %w", err)
	}

	return ctrl.Result{}, nil
}
