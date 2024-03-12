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
	"github.com/kyma-project/lifecycle-manager/pkg/log"
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
	logger.V(log.DebugLevel).Info("Purge reconciliation started")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted", req.NamespacedName))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed getting Kyma %s: %w", req.NamespacedName, err)
	}

	if kyma.DeletionTimestamp.IsZero() {
		if err := r.ensurePurgeFinalizer(ctx, kyma); err != nil {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Failed setting purge finalizer for Kyma %s: %s", req.NamespacedName, err))
			r.raiseSettingPurgeFinalizerFailedEvent(kyma, err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if requeueAfter := r.calculateRequeueAfterTime(kyma); requeueAfter != 0 {
		logger.Info(fmt.Sprintf("Purge reconciliation for Kyma %s will be requeued after %s", req.NamespacedName, requeueAfter))
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	start := time.Now()
	remoteClient, err := r.ResolveRemoteClient(ctx, client.ObjectKeyFromObject(kyma))
	if err != nil {
		if util.IsNotFound(err) {
			if err := r.dropPurgeFinalizer(ctx, kyma); err != nil {
				r.raiseRemovingPurgeFinalizerFailedError(ctx, kyma, err)
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed getting remote client for Kyma %s: %w", req.NamespacedName, err)
	}

	r.Metrics.UpdatePurgeCount()
	if err := r.performCleanup(ctx, remoteClient); err != nil {
		logger.Error(err, fmt.Sprintf("failed purge cleanup for Kyma %s", req.NamespacedName))
		r.Metrics.UpdatePurgeError(ctx, kyma, metrics.ErrCleanup)
		return ctrl.Result{}, err
	}

	if err := r.dropPurgeFinalizer(ctx, kyma); err != nil {
		r.raiseRemovingPurgeFinalizerFailedError(ctx, kyma, err)
		return ctrl.Result{}, err
	}

	r.Metrics.UpdatePurgeTime(time.Since(start))

	return ctrl.Result{}, nil
}

func (r *PurgeReconciler) UpdateStatus(ctx context.Context, kyma *v1beta2.Kyma, state shared.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		return fmt.Errorf("failed updating status to %s because of %s: %w", state, message, err)
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
		return fmt.Errorf("failed updating object: %w", err)
	}
	return nil
}

func (r *PurgeReconciler) dropPurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.ContainsFinalizer(kyma, shared.PurgeFinalizer) {
		controllerutil.RemoveFinalizer(kyma, shared.PurgeFinalizer)
		if err := r.Update(ctx, kyma); err != nil {
			return fmt.Errorf("failed updating object: %w", err)
		}
	}
	return nil
}

func (r *PurgeReconciler) calculateRequeueAfterTime(kyma *v1beta2.Kyma) time.Duration {
	deletionDeadline := kyma.DeletionTimestamp.Add(r.PurgeFinalizerTimeout)
	if time.Now().Before(deletionDeadline) {
		return time.Until(deletionDeadline.Add(time.Second))
	}
	return 0
}

func (r *PurgeReconciler) performCleanup(ctx context.Context, remoteClient client.Client) error {
	crdList := apiextensionsv1.CustomResourceDefinitionList{}
	if err := remoteClient.List(ctx, &crdList); err != nil {
		return fmt.Errorf("failed fetching CRDs from remote cluster: %w", err)
	}

	for _, crd := range crdList.Items {
		if shouldSkip(crd, r.SkipCRDs) {
			continue
		}

		staleResources, err := getAllRemainingCRs(ctx, remoteClient, crd)
		if err != nil {
			return fmt.Errorf("failed fetching stale resources from remote cluster: %w", err)
		}

		err = dropFinalizers(ctx, remoteClient, staleResources)
		if err != nil {
			return fmt.Errorf("failed removing finalizers from stale resources: %w", err)
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
		return unstructured.UnstructuredList{}, fmt.Errorf("failed fetching resources: %w", err)
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
			return fmt.Errorf("failed updating resource: %w", err)
		}
	}
	return nil
}

func (r *PurgeReconciler) raiseSettingPurgeFinalizerFailedEvent(kyma *v1beta2.Kyma, err error) {
	r.Event(kyma, "Warning", "SettingPurgeFinalizerFailed", err.Error())
}

func (r *PurgeReconciler) raiseRemovingPurgeFinalizerFailedEvent(kyma *v1beta2.Kyma, err error) {
	r.Event(kyma, "Warning", "RemovingPurgeFinalizerFailed", err.Error())
}

func (r *PurgeReconciler) raiseRemovingPurgeFinalizerFailedError(ctx context.Context, kyma *v1beta2.Kyma, err error) {
	logf.FromContext(ctx).Error(err, fmt.Sprintf("Failed removing purge finalizer from Kyma %s/%s", kyma.GetNamespace(), kyma.GetName()))
	r.raiseRemovingPurgeFinalizerFailedEvent(kyma, err)
	r.Metrics.UpdatePurgeError(ctx, kyma, metrics.ErrPurgeFinalizerRemoval)
}
