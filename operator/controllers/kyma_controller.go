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

package controllers

import (
	"context"
	"fmt"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/dynamic"
	"github.com/kyma-project/kyma-operator/operator/pkg/remote"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"time"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"

	"github.com/kyma-project/kyma-operator/operator/pkg/index"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"github.com/kyma-project/kyma-operator/operator/pkg/status"
	"github.com/kyma-project/kyma-operator/operator/pkg/util"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// RequeueSuccessInterval determines the duration after which an already successfully reconciled Kyma
// is enqueued for checking if it's still in a consistent state.
const RequeueSuccessInterval = 20 * time.Second

// RequeueFailureInterval determines the duration after which a failing reconciliation is retried and enqueued
// for a next try at recovering (e.g. because an Remote Synchronization Interaction failed)
const RequeueFailureInterval = 10 * time.Second

// RequeueWaitingInterval determines the duration after which a pending reconciliation is requeued if the operator
// determines that it needs to wait for a certain state to update before it can proceed
// (e.g. because of pending finalizers in the deletion process)
const RequeueWaitingInterval = 3 * time.Second

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	record.EventRecorder
}

func (r *KymaReconciler) GetEventRecorder() record.EventRecorder {
	return r.EventRecorder
}

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;onEvent;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates,verbs=get;list;watch;create;update;patch;onEvent;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	ctx = adapter.ContextWithRecorder(ctx, r.GetEventRecorder())

	// check if kyma resource exists
	kyma := operatorv1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, &kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != operatorv1alpha1.KymaStateDeleting {
		if err := remote.DeleteRemotelySyncedKyma(ctx, r.Client, client.ObjectKeyFromObject(&kyma)); client.IgnoreNotFound(err) != nil {
			return ctrl.Result{RequeueAfter: RequeueFailureInterval}, err
		}
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.updateKymaStatus(ctx, &kyma, operatorv1alpha1.KymaStateDeleting, "deletion timestamp set")
	}

	// check finalizer
	if !controllerutil.ContainsFinalizer(&kyma, labels.Finalizer) {
		controllerutil.AddFinalizer(&kyma, labels.Finalizer)
		return ctrl.Result{}, r.Update(ctx, &kyma)
	}

	syncContext, err := remote.InitializeKymaSynchronizationContext(ctx, r.Client, &kyma)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueFailureInterval}, err
	}
	remoteKyma, err := syncContext.CreateOrFetchRemoteKyma(ctx)
	if err != nil {
		return ctrl.Result{RequeueAfter: RequeueFailureInterval}, err
	}
	if synchronizationRequiresRequeue, err := syncContext.SynchronizeRemoteKyma(ctx, remoteKyma); err != nil || synchronizationRequiresRequeue {
		return ctrl.Result{RequeueAfter: RequeueFailureInterval}, err
	}

	// state handling
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateProcessing:
		return ctrl.Result{RequeueAfter: RequeueFailureInterval}, r.HandleProcessingState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateDeleting:
		if dependentsDeleting, err := r.HandleDeletingState(ctx, &logger, &kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{RequeueAfter: RequeueWaitingInterval}, nil
		}
	case operatorv1alpha1.KymaStateError:
		return ctrl.Result{RequeueAfter: RequeueWaitingInterval}, r.HandleErrorState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateReady:
		return ctrl.Result{RequeueAfter: RequeueSuccessInterval}, r.HandleReadyState(ctx, &logger, &kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) HandleInitialState(ctx context.Context, _ *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing, "initial state")
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	logger.Info("processing " + kyma.Name)

	// fetch templates
	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, "templates could not be fetched")
	}

	// reconcile from templates
	if err := r.ReconcileKymaForRelease(ctx, kyma, templates); err != nil {
		return err
	}

	// update status conditions
	updateRequired, err := r.checkAndUpdateComponentConditions(ctx, kyma, templates)
	if err != nil {
		return err
	}
	if updateRequired {
		return r.updateKymaStatus(ctx, kyma, kyma.Status.State, "updating component conditions")
	}

	// set ready condition if applicable
	if kyma.AreAllReadyConditionsSetForKyma() {
		message := fmt.Sprintf("reconciliation of %s finished!", kyma.Name)
		logger.Info(message)
		r.Event(kyma, "Normal", "ReconciliationSuccess", message)
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateReady, message)
	}

	return nil
}

func (r *KymaReconciler) HandleDeletingState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) (bool, error) {
	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		return false, fmt.Errorf("deletion cannot proceed - templates could not be fetched: %w", err)
	}

	for _, component := range kyma.Spec.Components {
		actualComponentStruct, err := util.GetUnstructuredComponentFromTemplate(templates, component.Name, kyma)
		if err != nil {
			return false, err
		}

		if err = r.Get(ctx, client.ObjectKeyFromObject(actualComponentStruct), actualComponentStruct); err == nil {
			// component CR still exists

			logger.Info(fmt.Sprintf("deletion cannot proceed - waiting for component CR %s to be deleted for %s",
				actualComponentStruct.GetName(), client.ObjectKeyFromObject(kyma)))
			return true, nil
		} else if !errors.IsNotFound(err) {
			// unknown error while getting component CR
			return false, fmt.Errorf("deletion cannot proceed - unknown error: %w", err)
		}
	}

	logger.Info("All component CRs have been removed, removing finalizer",
		"resource", client.ObjectKeyFromObject(kyma))

	if err := remote.RemoveFinalizerFromRemoteKyma(ctx, r, client.ObjectKeyFromObject(kyma)); client.IgnoreNotFound(err) != nil {
		return false, err
	}
	logger.Info("removed remote finalizer",
		"resource", client.ObjectKeyFromObject(kyma))

	// remove finalizer
	controllerutil.RemoveFinalizer(kyma, labels.Finalizer)
	return false, r.Update(ctx, kyma)
}

func (r *KymaReconciler) HandleErrorState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return r.HandleConsistencyChanges(ctx, logger, kyma)
}

func (r *KymaReconciler) HandleReadyState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return r.HandleConsistencyChanges(ctx, logger, kyma)
}

func (r *KymaReconciler) HandleConsistencyChanges(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	// outdated template
	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		logger.Error(err, "error fetching fetching templates")
		return status.KymaHandler(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateError, err.Error())
	}
	if release.AreTemplatesOutdated(logger, kyma, templates) {
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing, "template update")
	}

	// condition update on component CRs
	updateRequired, err := r.checkAndUpdateComponentConditions(ctx, kyma, templates)
	// TODO: separate error and update handling
	if err != nil || updateRequired {
		if err != nil {
			logger.Error(err, "error while updating component status conditions")
		}
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing,
			"updating component conditions")
	}

	// generation change
	if kyma.Status.ObservedGeneration != kyma.Generation {
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing,
			"object updated")
	}

	return nil
}

func (r *KymaReconciler) updateKymaStatus(ctx context.Context, kyma *operatorv1alpha1.Kyma, state operatorv1alpha1.KymaState, message string) error {
	return status.KymaHandler(r).UpdateStatus(ctx, kyma, state, message)
}

func (r *KymaReconciler) CreateOrUpdateComponentsFromTemplate(ctx context.Context, kymaObj *operatorv1alpha1.Kyma, templates release.TemplateLookupResultsByName) ([]util.ComponentsAssociatedWithTemplate, error) {
	kymaObjectKey := client.ObjectKey{Name: kymaObj.Name, Namespace: kymaObj.Namespace}
	namespacedName := kymaObjectKey.String()
	logger := log.FromContext(ctx).WithName(namespacedName)
	channel := kymaObj.Spec.Channel

	if len(kymaObj.Spec.Components) < 1 {
		return nil, fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	templatesOutdated := release.AreTemplatesOutdated(&logger, kymaObj, templates)

	var componentNamesAffected []util.ComponentsAssociatedWithTemplate
	for _, component := range kymaObj.Spec.Components {
		lookupResult := templates[component.Name]
		actualComponentStruct, err := util.GetUnstructuredComponentFromTemplate(templates, component.Name, kymaObj)
		if err != nil {
			return nil, err
		}
		err = r.Get(ctx, client.ObjectKeyFromObject(actualComponentStruct), actualComponentStruct)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}

		if errors.IsNotFound(err) {
			// merge template and component settings
			util.CopyComponentSettingsToUnstructuredFromResource(actualComponentStruct, component)

			// set labels
			util.SetComponentCRLabels(actualComponentStruct, component.Name, channel, kymaObj.Name)
			// set owner reference
			if err := controllerutil.SetOwnerReference(kymaObj, actualComponentStruct, r.Scheme()); err != nil {
				return nil, fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			// create resource if not found
			if err := r.Client.Create(ctx, actualComponentStruct, &client.CreateOptions{}); err != nil {
				return nil, fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully created component CR of", "type", component.Name, "templateGeneration", lookupResult.Template.GetGeneration())

			componentNamesAffected = append(componentNamesAffected, util.ComponentsAssociatedWithTemplate{
				ComponentName:      component.Name,
				TemplateGeneration: lookupResult.Template.GetGeneration(),
				TemplateChannel:    lookupResult.Template.Spec.Channel,
			})
		} else if templatesOutdated {
			condition, exists := status.KymaHandler(r).GetReadyConditionForComponent(kymaObj, component.Name)
			if !exists {
				return nil, fmt.Errorf("condition not found for component %s", component.Name)
			}

			// either the template in the condition is outdated (reflected by a generation change on the template)
			//or the template that is supposed to be applied changed (e.g. because the kyma spec changed)
			if condition.TemplateInfo.Generation != lookupResult.Template.GetGeneration() ||
				condition.TemplateInfo.Channel != lookupResult.Template.Spec.Channel {

				// merge template and component settings
				util.CopyComponentSettingsToUnstructuredFromResource(actualComponentStruct, component)

				// set labels
				util.SetComponentCRLabels(actualComponentStruct, component.Name, channel, kymaObj.Name)

				// update the spec
				actualComponentStruct.Object["spec"] = lookupResult.Template.Spec.Data.Object["spec"]

				if err := r.Client.Update(ctx, actualComponentStruct, &client.UpdateOptions{}); err != nil {
					return nil, fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
				}

				logger.Info("successfully updated component cr", "type", component.Name, "templateGeneration", lookupResult.Template.GetGeneration())
				componentNamesAffected = append(componentNamesAffected, util.ComponentsAssociatedWithTemplate{
					ComponentName:      component.Name,
					TemplateGeneration: lookupResult.Template.GetGeneration(),
					TemplateChannel:    lookupResult.Template.Spec.Channel,
				})
			}
		}
	}
	return componentNamesAffected, nil
}

func (r *KymaReconciler) ReconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma, templates release.TemplateLookupResultsByName) error {
	logger := log.FromContext(ctx)
	affectedComponents, err := r.CreateOrUpdateComponentsFromTemplate(ctx, kyma, templates)

	if err != nil {
		message := fmt.Sprintf("Component CR creation error: %s", err.Error())
		logger.Info(message)
		r.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))
		statusErr := r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message)
		if statusErr != nil {
			return statusErr
		}
		return err
	}

	if len(affectedComponents) > 0 {
		// check component conditions, if not present add them
		logger.Info("checking condition for component CRs")
		status.KymaHandler(r).AddReadyConditionForObjects(kyma, affectedComponents, operatorv1alpha1.ConditionStatusFalse, "initial condition for component CR")
		release.New(kyma, ctx).IssueChannelChangeInProgress()
		return r.updateKymaStatus(ctx, kyma, kyma.Status.State, "component conditions updated")
	}

	return nil
}

func (r *KymaReconciler) checkAndUpdateComponentConditions(ctx context.Context, kyma *operatorv1alpha1.Kyma, templates release.TemplateLookupResultsByName) (bool, error) {
	updateRequired := false
	for _, component := range kyma.Spec.Components {

		actualComponentStruct, err := util.GetUnstructuredComponentFromTemplate(templates, component.Name, kyma)
		if err != nil {
			return false, err
		}

		if err = r.Get(ctx, client.ObjectKeyFromObject(actualComponentStruct), actualComponentStruct); err != nil {
			return false, err
		}

		updated, err := status.KymaHandler(r).UpdateComponentConditions(actualComponentStruct, kyma)
		if err != nil {
			return false, err
		}

		if !updateRequired {
			// only set once and do not overwrite
			updateRequired = updated
		}
	}
	return updateRequired, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&operatorv1alpha1.Kyma{}).WithOptions(options)

	if dynamicInformers, err := dynamic.Informers(mgr); err != nil {
		return err
	} else {
		for _, informer := range dynamicInformers {
			controllerBuilder = controllerBuilder.
				Watches(informer, &handler.Funcs{UpdateFunc: watch.NewComponentChangeHandler(r).Watch(context.TODO())},
					builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
		}
	}

	controllerBuilder = controllerBuilder.Watches(
		&source.Kind{Type: &operatorv1alpha1.ModuleTemplate{}},
		handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch(context.TODO())),
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))

	// here we define a watch on secrets for the kyma operator so that the cache is picking up changes
	controllerBuilder = controllerBuilder.Watches(&source.Kind{Type: &v1.Secret{}}, handler.Funcs{})

	if err := index.TemplateChannel().With(context.TODO(), mgr.GetFieldIndexer()); err != nil {
		return err
	}

	return controllerBuilder.Complete(r)
}
