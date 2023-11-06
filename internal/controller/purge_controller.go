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

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/controller/purge/metrics"
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
}

//nolint:funlen
func (r *PurgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.InfoLevel).Info("Purge Reconciliation started")

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if !util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted", req.NamespacedName))
			return ctrl.Result{}, fmt.Errorf("purgeController: %w", err)
		}
		return ctrl.Result{Requeue: false}, nil
	}

	if kyma.DeletionTimestamp.IsZero() {
		if err := r.ensurePurgeFinalizer(ctx, kyma); err != nil {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Updating purge finalizers for Kyma  %s failed with err %s",
				req.NamespacedName, err))
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	deletionDeadline := kyma.DeletionTimestamp.Add(r.PurgeFinalizerTimeout)
	if time.Now().Before(deletionDeadline) {
		requeueAfter := time.Until(deletionDeadline.Add(time.Second))
		logger.V(log.DebugLevel).Info(fmt.Sprintf("Purge reconciliation for Kyma  %s will be requeued after %s",
			req.NamespacedName, requeueAfter))
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	start := time.Now()
	remoteClient, err := r.ResolveRemoteClient(ctx, client.ObjectKeyFromObject(kyma))
	if util.IsNotFound(err) {
		if err := r.dropPurgeFinalizer(ctx, kyma); err != nil {
			logger.Error(err, "Couldn't remove Purge Finalizer from the Kyma object")
			if err := metrics.UpdatePurgeError(kyma, metrics.ErrPurgeFinalizerRemoval); err != nil {
				logger.Error(err, "Failed to update error metrics")
			}
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	metrics.UpdatePurgeCount()
	if err := r.performCleanup(ctx, remoteClient); err != nil {
		logger.Error(err, "Finalizer Purging failed")
		if err := metrics.UpdatePurgeError(kyma, metrics.ErrCleanup); err != nil {
			logger.Error(err, "Failed to update error metrics")
		}
		return ctrl.Result{}, err
	}

	if err := r.dropPurgeFinalizer(ctx, kyma); err != nil {
		logger.Error(err, "Couldn't remove Purge Finalizer from the Kyma object")
		if err := metrics.UpdatePurgeError(kyma, metrics.ErrPurgeFinalizerRemoval); err != nil {
			logger.Error(err, "Failed to update error metrics")
		}
		return ctrl.Result{}, err
	}
	duration := time.Since(start)
	metrics.UpdatePurgeTime(duration)

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
	if controllerutil.ContainsFinalizer(kyma, v1beta2.PurgeFinalizer) {
		return nil
	}
	controllerutil.AddFinalizer(kyma, v1beta2.PurgeFinalizer)
	if err := r.Update(ctx, kyma); err != nil {
		err = fmt.Errorf("failed to add purge finalizer: %w", err)
		r.setFinalizerWarningEvent(kyma, err)
		return err
	}
	return nil
}

func (r *PurgeReconciler) dropPurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.ContainsFinalizer(kyma, v1beta2.PurgeFinalizer) {
		controllerutil.RemoveFinalizer(kyma, v1beta2.PurgeFinalizer)
		if err := r.Update(ctx, kyma); err != nil {
			err = fmt.Errorf("failed to remove purge finalizer: %w", err)
			r.setFinalizerWarningEvent(kyma, err)
			return err
		}
	}
	return nil
}

func (r *PurgeReconciler) performCleanup(ctx context.Context, remoteClient client.Client) error {
	crdList := apiextensions.CustomResourceDefinitionList{}
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

func shouldSkip(crd apiextensions.CustomResourceDefinition, matcher matcher.CRDMatcherFunc) bool {
	if crd.Spec.Group == v1beta2.GroupVersion.Group && crd.Spec.Names.Kind == string(v1beta2.KymaKind) {
		return true
	}
	return matcher(crd)
}

func getAllRemainingCRs(ctx context.Context, remoteClient client.Client,
	crd apiextensions.CustomResourceDefinition,
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
