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
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/metrics"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/matcher"
	"github.com/kyma-project/lifecycle-manager/pkg/status"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type RemoteClientResolver func(context.Context, client.ObjectKey) (client.Client, error)

type PurgeReconciler struct {
	client.Client
	record.EventRecorder
	ResolveRemoteClient   RemoteClientResolver
	PurgeFinalizerTimeout time.Duration
	SkipCRDs              matcher.CRDMatcherFunc
	IsManagedKyma         bool
	Metrics               *metrics.PurgeMetrics
}

func (r *PurgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Purge reconciliation started")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Kyma %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{Requeue: false}, nil
		}
		return ctrl.Result{}, fmt.Errorf("purgeController: %w", err)
	}

	if kyma.DeletionTimestamp.IsZero() {
		err := r.ensurePurgeFinalizer(ctx, kyma)
		if err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: false}, nil
	}

	requeueAfter := r.calculateRequeueAfterTime(ctx, kyma)
	if requeueAfter != 0 {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	start := time.Now()
	remoteClient, err := r.ResolveRemoteClient(ctx, client.ObjectKeyFromObject(kyma))
	if util.IsNotFound(err) {
		if err := r.dropPurgeFinalizer(ctx, kyma); err != nil {
			r.Metrics.UpdatePurgeError(ctx, kyma, metrics.ErrPurgeFinalizerRemoval)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	r.Metrics.UpdatePurgeCount()
	if err := r.performCleanup(ctx, remoteClient); err != nil {
		logger.Error(err, "Purge Cleanup failed")
		r.Metrics.UpdatePurgeError(ctx, kyma, metrics.ErrCleanup)
		return ctrl.Result{}, err
	}

	if err := r.dropPurgeFinalizer(ctx, kyma); err != nil {
		r.Metrics.UpdatePurgeError(ctx, kyma, metrics.ErrPurgeFinalizerRemoval)
		return ctrl.Result{}, err
	}
	duration := time.Since(start)
	r.Metrics.UpdatePurgeTime(duration)

	logger.Info("Purge reconciliation done")
	return ctrl.Result{}, nil
}

func (r *PurgeReconciler) UpdateStatus(ctx context.Context, kyma *v1beta2.Kyma, state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *PurgeReconciler) UpdateMetrics(_ context.Context, _ *v1beta2.Kyma) {}

func (r *PurgeReconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}

func (r *PurgeReconciler) ensurePurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.ContainsFinalizer(kyma, shared.PurgeFinalizer) {
		return nil
	}
	controllerutil.AddFinalizer(kyma, shared.PurgeFinalizer)
	if err := r.Update(ctx, kyma); err != nil {
		err = fmt.Errorf("failed to add purge finalizer: %w", err)
		logf.FromContext(ctx).Info(
			fmt.Sprintf("Updating purge finalizers for Kyma  %s/%s failed with err %s",
				kyma.Namespace, kyma.Name, err))
		r.setFinalizerWarningEvent(kyma, err)
		return err
	}
	return nil
}

func (r *PurgeReconciler) dropPurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.ContainsFinalizer(kyma, shared.PurgeFinalizer) {
		controllerutil.RemoveFinalizer(kyma, shared.PurgeFinalizer)
		if err := r.Update(ctx, kyma); err != nil {
			err = fmt.Errorf("failed to remove purge finalizer: %w", err)
			r.setFinalizerWarningEvent(kyma, err)
			logf.FromContext(ctx).Error(err, "Couldn't remove Purge Finalizer from the Kyma object")
			return err
		}
	}
	return nil
}

func (r *PurgeReconciler) calculateRequeueAfterTime(ctx context.Context, kyma *v1beta2.Kyma) time.Duration {
	deletionDeadline := kyma.DeletionTimestamp.Add(r.PurgeFinalizerTimeout)
	if time.Now().Before(deletionDeadline) {
		requeueAfter := time.Until(deletionDeadline.Add(time.Second))
		logf.FromContext(ctx).Info(fmt.Sprintf("Purge reconciliation for Kyma  %s/%s will be requeued after %s",
			kyma.Namespace, kyma.Namespace, requeueAfter))
		return requeueAfter
	}
	return 0
}

func (r *PurgeReconciler) performCleanup(ctx context.Context, remoteClient client.Client) error {
	crdList := apiextensionsv1.CustomResourceDefinitionList{}
	if err := remoteClient.List(ctx, &crdList); err != nil {
		return fmt.Errorf("failed to fetch CRDs from remote cluster: %w", err)
	}

	for _, crd := range crdList.Items {
		if shouldSkip(crd, r.SkipCRDs) {
			continue
		}

		staleResources, err := getAllRemainingCRs(ctx, remoteClient, crd)
		if err != nil {
			return fmt.Errorf("failed to fetch stale resources from remote cluster: %w", err)
		}

		err = dropFinalizers(ctx, remoteClient, staleResources)
		if err != nil {
			return fmt.Errorf("failed to purge stale resources: %w", err)
		}
	}

	return nil
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
		return unstructured.UnstructuredList{}, fmt.Errorf("failed to fetch stale resources: %w", err)
	}

	return staleResources, nil
}

func dropFinalizers(ctx context.Context, remoteClient client.Client,
	staleResources unstructured.UnstructuredList,
) error {
	for index := range staleResources.Items {
		resource := staleResources.Items[index]
		resource.SetFinalizers(nil)
		if err := remoteClient.Update(ctx, &resource); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
	}
	return nil
}

func (r *PurgeReconciler) setFinalizerWarningEvent(kyma *v1beta2.Kyma, err error) {
	r.Event(kyma, "Warning", "SettingPurgeFinalizerError", err.Error())
}
