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

package purge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/internal/remote"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/matcher"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	setFinalizerFailure    event.Reason = "SettingPurgeFinalizerFailed"
	removeFinalizerFailure event.Reason = "RemovingPurgeFinalizerFailed"
)

type Reconciler struct {
	client.Client
	event.Event

	SkrContextFactory     remote.SkrContextProvider
	PurgeFinalizerTimeout time.Duration
	SkipCRDs              matcher.CRDMatcherFunc
	IsManagedKyma         bool
	Metrics               *metrics.PurgeMetrics
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Purge reconciliation started")

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		return handleKymaNotFoundError(logger, kyma, err)
	}

	if kyma.DeletionTimestamp.IsZero() {
		return r.handleKymaNotMarkedForDeletion(ctx, kyma)
	}

	if requeueAfter := r.calculateRequeueAfterTime(kyma); requeueAfter != 0 {
		return handlePurgeNotDue(logger, kyma, requeueAfter)
	}

	start := time.Now()
	err := r.SkrContextFactory.Init(ctx, kyma.GetNamespacedName())
	if err != nil {
		return r.handleSkrNotFoundError(ctx, kyma, err)
	}

	skrContext, err := r.SkrContextFactory.Get(kyma.GetNamespacedName())
	if err != nil {
		return r.handleSkrNotFoundError(ctx, kyma, err)
	}

	return r.handlePurge(ctx, kyma, skrContext.Client, start)
}

func (r *Reconciler) UpdateStatus(ctx context.Context, kyma *v1beta2.Kyma, state shared.State, message string) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		return fmt.Errorf("failed updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *Reconciler) UpdateMetrics(_ context.Context, _ *v1beta2.Kyma) {}

func (r *Reconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}

func handleKymaNotFoundError(logger logr.Logger, kyma *v1beta2.Kyma, err error) (ctrl.Result, error) {
	if util.IsNotFound(err) {
		logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted", kyma.GetName()))
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, fmt.Errorf("failed getting Kyma %s: %w", kyma.GetName(), err)
}

func (r *Reconciler) handleKymaNotMarkedForDeletion(ctx context.Context, kyma *v1beta2.Kyma) (ctrl.Result, error) {
	if err := r.ensurePurgeFinalizer(ctx, kyma); err != nil {
		logf.FromContext(ctx).
			V(log.DebugLevel).
			Info(fmt.Sprintf("Failed setting purge finalizer for Kyma %s: %s", kyma.GetName(), err))
		r.Warning(kyma, setFinalizerFailure, err)
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func handlePurgeNotDue(logger logr.Logger, kyma *v1beta2.Kyma, requeueAfter time.Duration) (ctrl.Result, error) {
	logger.Info(fmt.Sprintf("Purge reconciliation for Kyma %s will be requeued after %s", kyma.GetName(), requeueAfter))
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) handleRemovingPurgeFinalizerFailedError(
	ctx context.Context,
	kyma *v1beta2.Kyma,
	err error,
) (ctrl.Result, error) {
	logf.FromContext(ctx).
		Error(err, fmt.Sprintf("Failed removing purge finalizer from Kyma %s/%s", kyma.GetNamespace(), kyma.GetName()))
	r.Warning(kyma, removeFinalizerFailure, err)
	r.Metrics.SetPurgeError(ctx, kyma, metrics.ErrPurgeFinalizerRemoval)
	return ctrl.Result{}, err
}

func (r *Reconciler) handleSkrNotFoundError(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result, error) {
	if util.IsNotFound(err) {
		dropped, err := r.dropPurgeFinalizer(ctx, kyma)
		if err != nil {
			return r.handleRemovingPurgeFinalizerFailedError(ctx, kyma, err)
		}
		if dropped {
			logf.FromContext(ctx).Info("Removed purge finalizer for Kyma " + kyma.GetName())
		}
		r.Metrics.DeletePurgeError(ctx, kyma, metrics.ErrPurgeFinalizerRemoval)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, fmt.Errorf("failed getting remote client for Kyma %s: %w", kyma.GetName(), err)
}

func (r *Reconciler) handleCleanupError(ctx context.Context, kyma *v1beta2.Kyma, err error) (ctrl.Result, error) {
	logf.FromContext(ctx).Error(err, "failed purge cleanup for Kyma "+kyma.GetName())
	r.Metrics.SetPurgeError(ctx, kyma, metrics.ErrCleanup)
	return ctrl.Result{}, err
}

func (r *Reconciler) handlePurge(
	ctx context.Context,
	kyma *v1beta2.Kyma,
	remoteClient client.Client,
	start time.Time,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	r.Metrics.UpdatePurgeCount()

	handledResources, err := r.performCleanup(ctx, remoteClient)
	if len(handledResources) > 0 {
		logger.Info(
			fmt.Sprintf(
				"Removed all finalizers for Kyma %s related resources %s",
				kyma.GetName(),
				strings.Join(handledResources, ", "),
			),
		)
	}
	if err != nil {
		return r.handleCleanupError(ctx, kyma, err)
	}
	r.Metrics.DeletePurgeError(ctx, kyma, metrics.ErrCleanup)

	dropped, err := r.dropPurgeFinalizer(ctx, kyma)
	if dropped {
		logger.Info("Removed purge finalizer for Kyma " + kyma.GetName())
	}
	if err != nil {
		return r.handleRemovingPurgeFinalizerFailedError(ctx, kyma, err)
	}
	r.Metrics.DeletePurgeError(ctx, kyma, metrics.ErrPurgeFinalizerRemoval)

	r.Metrics.UpdatePurgeTime(time.Since(start))
	return ctrl.Result{}, nil
}

func (r *Reconciler) ensurePurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.AddFinalizer(kyma, shared.PurgeFinalizer) {
		if err := r.Update(ctx, kyma); err != nil {
			return fmt.Errorf("failed updating object: %w", err)
		}
	}
	return nil
}

func (r *Reconciler) dropPurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) (bool, error) {
	if controllerutil.RemoveFinalizer(kyma, shared.PurgeFinalizer) {
		if err := r.Update(ctx, kyma); err != nil {
			return false, fmt.Errorf("failed updating object: %w", err)
		}
		return true, nil
	}
	return false, nil
}

func (r *Reconciler) calculateRequeueAfterTime(kyma *v1beta2.Kyma) time.Duration {
	deletionDeadline := kyma.DeletionTimestamp.Add(r.PurgeFinalizerTimeout)
	if time.Now().Before(deletionDeadline) {
		return time.Until(deletionDeadline.Add(time.Second))
	}
	return 0
}

func (r *Reconciler) performCleanup(ctx context.Context, remoteClient client.Client) ([]string, error) {
	crdList := apiextensionsv1.CustomResourceDefinitionList{}
	if err := remoteClient.List(ctx, &crdList); err != nil {
		return nil, fmt.Errorf("failed fetching CRDs from remote cluster: %w", err)
	}

	var allHandledResources []string
	for _, crd := range crdList.Items {
		if shouldSkip(crd, r.SkipCRDs) {
			continue
		}

		staleResources, err := getAllRemainingCRs(ctx, remoteClient, crd)
		if err != nil {
			return allHandledResources, fmt.Errorf("failed fetching stale resources from remote cluster: %w", err)
		}

		handledResources, err := dropFinalizers(ctx, remoteClient, staleResources)
		if err != nil {
			return allHandledResources, fmt.Errorf("failed removing finalizers from stale resources: %w", err)
		}

		allHandledResources = append(allHandledResources, handledResources...)
	}

	return allHandledResources, nil
}

func shouldSkip(crd apiextensionsv1.CustomResourceDefinition, matcher matcher.CRDMatcherFunc) bool {
	if crd.Spec.Group == v1beta2.GroupVersion.Group && crd.Spec.Names.Kind == string(shared.KymaKind) {
		return true
	}
	return matcher(crd)
}

func getAllRemainingCRs(ctx context.Context, remoteClient client.Client,
	crd apiextensionsv1.CustomResourceDefinition,
) (unstructured.UnstructuredList, error) {
	staleResources := unstructured.UnstructuredList{}

	// Since there are multiple possible versions, we are choosing storage version
	var gvk schema.GroupVersionKind
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			gvk = schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Kind:    crd.Spec.Names.Kind,
				Version: version.Name,
			}
			break
		}
	}
	staleResources.SetGroupVersionKind(gvk)

	if err := remoteClient.List(ctx, &staleResources); err != nil {
		return unstructured.UnstructuredList{}, fmt.Errorf("failed fetching resources: %w", err)
	}

	return staleResources, nil
}

func dropFinalizers(ctx context.Context, remoteClient client.Client,
	staleResources unstructured.UnstructuredList,
) ([]string, error) {
	handledResources := make([]string, 0, len(staleResources.Items))
	for index := range staleResources.Items {
		resource := staleResources.Items[index]
		resource.SetFinalizers(nil)
		if err := remoteClient.Update(ctx, &resource); err != nil {
			return handledResources, fmt.Errorf("failed updating resource: %w", err)
		}
		handledResources = append(handledResources, fmt.Sprintf("%s/%s", resource.GetNamespace(), resource.GetName()))
	}
	return handledResources, nil
}
