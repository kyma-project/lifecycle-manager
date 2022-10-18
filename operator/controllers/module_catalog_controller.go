package controllers

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/operator/pkg/remote"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/catalog"
)

type ModuleCatalogReconciler struct {
	client.Client
	record.EventRecorder
	RequeueIntervals
	RemoteClientCache *remote.ClientCache
}

//nolint:lll
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates/finalizers,verbs=update

func (r *ModuleCatalogReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Catalog Sync loop starting for", "resource", req.NamespacedName.String())
	catalogSync := catalog.NewSync(r.Client, r.EventRecorder, r.RemoteClientCache, catalog.Settings{})

	// check if kyma resource exists
	kyma := &v1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		if k8serrors.IsNotFound(err) {
			logger.Info(req.NamespacedName.String() + " got deleted!")
			return ctrl.Result{}, catalogSync.Cleanup(ctx)
		}

		return ctrl.Result{}, err //nolint:wrapcheck
	}

	moduleTemplateList := &v1alpha1.ModuleTemplateList{}
	err := r.List(ctx, moduleTemplateList, &client.ListOptions{})
	if err != nil {
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
	}

	if err := catalogSync.Run(ctx, kyma, moduleTemplateList); err != nil {
		kyma.UpdateCondition(v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionFalse)
		if err := r.Status().Update(ctx, kyma); err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
		}
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
	}

	kyma.UpdateCondition(v1alpha1.ConditionReasonModuleCatalogIsReady, metav1.ConditionTrue)
	if err := r.Status().Update(ctx, kyma); err != nil {
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
	}

	return ctrl.Result{}, nil
}
