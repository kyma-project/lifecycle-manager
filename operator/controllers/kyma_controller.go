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
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/remote"
	"github.com/kyma-project/kyma-operator/operator/pkg/signature"

	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"github.com/kyma-project/kyma-operator/operator/pkg/status"
)

var ErrNoComponentSpecified = errors.New("no component specified")

type RequeueIntervals struct {
	Success time.Duration
	Failure time.Duration
	Waiting time.Duration
}

// KymaReconciler reconciles a Kyma object.
type KymaReconciler struct {
	client.Client
	record.EventRecorder
	RequeueIntervals
	signature.VerificationSettings
	RateQPS   int
	RateBurst int
}

//nolint:lll
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;onEvent;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch;get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates,verbs=get;list;watch;create;update;patch;onEvent;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=moduletemplates/finalizers,verbs=update
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) { 
	logger := log.FromContext(ctx)
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	// check if kyma resource exists
	kyma := &v1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")

		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != v1alpha1.StateDeleting {
		if err := r.TriggerKymaDeletion(ctx, kyma); err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
		}

		// if the status is not yet set to deleting, also update the status
		if err := status.Helper(r).UpdateStatusForExistingModules(
			ctx, kyma, v1alpha1.StateDeleting, "deletion timestamp set",
		); err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, fmt.Errorf(
				"could not update kyma status after triggering deletion: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// check finalizer
	if kyma.CheckLabelsAndFinalizers() {
		if err := r.Update(ctx, kyma); err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure},
				fmt.Errorf("could not update kyma after finalizer check: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// create a remote synchronization context, and update the remote kyma with the state of the control plane
	if kyma.Spec.Sync.Enabled {
		err := r.synchronizeRemote(ctx, kyma)
		if err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
		}
	}

	if len(kyma.Spec.Modules) < 1 {
		return ctrl.Result{}, r.UpdateStatusFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error parsing %s: %w", kyma.Name, ErrNoComponentSpecified))
	}

	// state handling
	return r.stateHandling(ctx, kyma)
}

// synchronizeRemote replaces the given pointer to the Kyma Instance with an instance that contains the merged
// specification of the Control Plane and the Runtime.
func (r *KymaReconciler) synchronizeRemote(ctx context.Context, kyma *v1alpha1.Kyma) error {
	if kyma.Status.State == v1alpha1.StateDeleting {
		return nil
	}
	syncContext, err := remote.InitializeKymaSynchronizationContext(ctx, r.Client, kyma)
	if err != nil {
		return fmt.Errorf("could not initialize remote context before updating remote kyma: %w", err)
	}
	remoteKyma, err := syncContext.CreateOrFetchRemoteKyma(ctx)
	if err != nil {
		return fmt.Errorf("could not fetch kyma updating remote kyma: %w", err)
	}
	if err := syncContext.SynchronizeRemoteKyma(ctx, remoteKyma); err != nil {
		return fmt.Errorf("could not synchronize remote kyma: %w", err)
	}

	syncContext.ReplaceWithVirtualKyma(kyma, remoteKyma)

	return nil
}

func (r *KymaReconciler) stateHandling(ctx context.Context, kyma *v1alpha1.Kyma) (ctrl.Result, error) {
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, kyma)
	case v1alpha1.StateProcessing:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, r.HandleProcessingState(ctx, kyma)
	case v1alpha1.StateDeleting:
		if dependentsDeleting, err := r.HandleDeletingState(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Waiting}, nil
		}
	case v1alpha1.StateError:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Waiting}, r.HandleErrorState(ctx, kyma)
	case v1alpha1.StateReady:
		// TODO Adjust again
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, r.HandleReadyState(ctx, kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) HandleInitialState(ctx context.Context, kyma *v1alpha1.Kyma) error {
	kyma.MatchConditionsToModules()
	return r.UpdateStatus(ctx, kyma, v1alpha1.StateProcessing, "initial state")
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	logger.Info("processing " + kyma.Name)

	var err error
	var modules parsed.Modules
	// these are the actual modules
	modules, err = r.GetModules(ctx, kyma)
	if err != nil {
		return r.UpdateStatusFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error while fetching modules during processing: %w", err))
	}

	statusUpdateRequiredFromCreation, err := r.CreateOrUpdateModules(ctx, kyma, modules)
	if err != nil {
		return r.UpdateStatusFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("ParsedModule CR creation/update error: %w", err))
	}

	// Now we track the conditions: update the status based on their state
	// technically we could also update the state in the previous step alone determine if we are ready based on this

	statusUpdateRequiredFromSync, err := r.SyncConditionsWithModuleStates(ctx, kyma, modules)
	if err != nil {
		return r.UpdateStatusFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error while syncing conditions during processing: %w", err))
	}

	status.Helper(r).SyncModuleInfo(kyma, modules)

	statusUpdateRequiredFromDeletion := r.UpdateStatusModuleInfos(ctx, kyma)
	moduleInfos := kyma.GetNoLongerExistingModuleInfos()
	err = r.DeleteNoLongerExistingModules(ctx, moduleInfos)
	if err != nil {
		return r.UpdateStatusFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error while syncing conditions during deleting non exists modules: %w", err))
	}

	// set ready condition if applicable
	if kyma.AreAllConditionsReadyForKyma() {
		message := fmt.Sprintf("reconciliation of %s finished!", kyma.Name)
		logger.Info(message)
		r.Event(kyma, "Normal", "ReconciliationSuccess", message)

		return r.UpdateStatus(ctx, kyma, v1alpha1.StateReady, message)
	}

	// if the ready condition is not applicable, but we changed the conditions, we still need to issue an update
	if statusUpdateRequiredFromCreation || statusUpdateRequiredFromSync || statusUpdateRequiredFromDeletion {
		if err := status.Helper(r).UpdateStatusForExistingModules(
			ctx, kyma, kyma.Status.State, "updating component conditions"); err != nil {
			return fmt.Errorf("error while updating status for condition change: %w", err)
		}
		return nil
	}

	return nil
}

func (r *KymaReconciler) HandleDeletingState(ctx context.Context, kyma *v1alpha1.Kyma) (bool, error) {
	logger := log.FromContext(ctx)

	if kyma.Spec.Sync.Enabled {
		if err := remote.RemoveFinalizerFromRemoteKyma(ctx, r, kyma); client.IgnoreNotFound(err) != nil {
			return false, fmt.Errorf("error while trying to remove finalizer from remote: %w", err)
		}
		logger.Info("removed remote finalizer",
			"resource", client.ObjectKeyFromObject(kyma))
	}

	controllerutil.RemoveFinalizer(kyma, v1alpha1.Finalizer)

	if err := r.Update(ctx, kyma); err != nil {
		return false, fmt.Errorf("error while trying to udpate kyma during deletion: %w", err)
	}

	return false, nil
}

func (r *KymaReconciler) HandleErrorState(ctx context.Context, kyma *v1alpha1.Kyma) error {
	return r.HandleConsistencyChanges(ctx, kyma)
}

func (r *KymaReconciler) HandleReadyState(ctx context.Context, kyma *v1alpha1.Kyma) error {
	return r.HandleConsistencyChanges(ctx, kyma)
}

func (r *KymaReconciler) HandleConsistencyChanges(ctx context.Context, kyma *v1alpha1.Kyma) error {
	// condition update on CRs

	var err error
	var modules parsed.Modules
	// these are the actual modules
	modules, err = r.GetModules(ctx, kyma)
	if err != nil {
		return r.UpdateStatusFromErr(ctx, kyma, v1alpha1.StateError,
			fmt.Errorf("error while fetching modules during consistency check: %w", err))
	}

	status.Helper(r).SyncModuleInfo(kyma, modules)

	statusUpdateRequired, err := r.SyncConditionsWithModuleStates(ctx, kyma, modules)
	if err != nil {
		return fmt.Errorf("error while updating component status conditions: %w", err)
	}

	// at least one condition changed during the sync
	if statusUpdateRequired {
		return r.UpdateStatus(ctx, kyma, v1alpha1.StateProcessing,
			"updating component conditions")
	}

	// generation change
	if kyma.Status.ObservedGeneration != kyma.Generation {
		return r.UpdateStatus(ctx, kyma, v1alpha1.StateProcessing,
			"object updated")
	}

	for _, module := range modules {
		if module.TemplateOutdated {
			return r.UpdateStatus(ctx, kyma, v1alpha1.StateProcessing,
				fmt.Sprintf("module template of module %s got updated", module.Name))
		}
	}

	if len(kyma.GetNoLongerExistingModuleInfos()) > 0 {
		return r.UpdateStatus(ctx, kyma, v1alpha1.StateProcessing, "some module get deleted")
	}
	return nil
}

func (r *KymaReconciler) SyncConditionsWithModuleStates(ctx context.Context, kyma *v1alpha1.Kyma,
	modules parsed.Modules,
) (bool, error) {
	// Now we track the conditions: update the status based on their state
	statusUpdateRequired := kyma.FilterNotExistsConditions()

	var err error
	// Now, iterate through each module and compare the fitting condition in the Kyma CR to the state of the module
	for name, module := range modules {
		var conditionsUpdated bool
		// Next, we fetch it from the API Server to determine its status
		if err = r.getModule(ctx, module.Unstructured); client.IgnoreNotFound(err) != nil {
			break
		}
		// Finally, we update the condition based on its state and remember if we had to do an update
		// if the component is not found, we will have to go back into processing to try and rebuild it
		if k8serrors.IsNotFound(err) {
			conditionsUpdated = true
			err = nil
		} else if conditionsUpdated, err = status.Helper(r).
			UpdateConditionFromComponentState(name, module, kyma); err != nil {
			// if the component update failed we have to stop and retry until it succeeded
			break
		}

		if !statusUpdateRequired {
			// if any given condition was updated, we have to make sure to trigger a status update later on
			statusUpdateRequired = conditionsUpdated
		}
	}

	if err != nil {
		return statusUpdateRequired, fmt.Errorf("error occurred while synchronizing conditions with states: %w", err)
	}

	return statusUpdateRequired, nil
}

// CreateOrUpdateModules takes care of using the input module to acquire a new desired state in the control plane.
// either the template in the condition is outdated (reflected by a generation change on the template)
// or the template that is supposed to be applied changed (e.g. because the kyma spec changed)
// Here we fetch the latest status from the cluster since our module hasn't actually been verified before.
// This allows us to verify if the module has been present before or not.
func (r *KymaReconciler) CreateOrUpdateModules(ctx context.Context, kyma *v1alpha1.Kyma,
	modules parsed.Modules,
) (bool, error) {
	baseLogger := log.FromContext(ctx).WithName(client.ObjectKey{Name: kyma.Name, Namespace: kyma.Namespace}.String())
	for name, module := range modules {
		logger := module.Logger(baseLogger)
		syncCondition := func(message string) {
			status.Helper(r).SyncReadyConditionForModules(kyma, parsed.Modules{name: module},
				v1alpha1.ConditionStatusFalse, message)
		}
		create := func() (bool, error) {
			logger.Info("module not found, attempting to create it...")
			err := r.CreateModule(ctx, name, kyma, module)
			if err != nil {
				return false, err
			}
			syncCondition(fmt.Sprintf("initial condition for module %s", module.Name))
			logger.Info("successfully created module CR")
			return true, nil
		}
		update := func() (bool, error) {
			if err := r.UpdateModule(ctx, name, kyma, module); err != nil {
				return false, err
			}
			logger.Info("successfully updated module CR")
			syncCondition(fmt.Sprintf("updated condition for module %s", module.Name))
			return true, nil
		}

		unstructuredFromServer := initModuleFromServer(module)
		err := r.getModule(ctx, unstructuredFromServer)
		if k8serrors.IsNotFound(err) {
			return create()
		} else if err != nil {
			return false, fmt.Errorf("error occurred while fetching module %s: %w", module.GetName(), err)
		}

		module.UpdateModuleFromCluster(unstructuredFromServer)

		if module.TemplateOutdated {
			condition, _ := status.Helper(r).GetReadyConditionForComponent(kyma, name)
			if module.StateMismatchedWithCondition(condition) {
				return update()
			}
		}

		// if we have NO create, NO update,the template was NOT outdated and the Condition did not exist yet.
		// either the condition was never tracked before, or it was tracked before but isn't ready yet.
		// we now insert the condition to false as we expect the next step to verify every time if the module is still ready,
		// by default the module will NOT be ready.
		status.Helper(r).SyncReadyConditionForModules(kyma, parsed.Modules{name: module}, v1alpha1.ConditionStatusFalse,
			fmt.Sprintf("module %s was not created or updated", module.Name))
	}

	return false, nil
}

func initModuleFromServer(module *parsed.Module) *unstructured.Unstructured {
	unstructuredFromServer := unstructured.Unstructured{}
	unstructuredFromServer.SetGroupVersionKind(module.Unstructured.GroupVersionKind())
	unstructuredFromServer.SetNamespace(module.Unstructured.GetNamespace())
	unstructuredFromServer.SetName(module.Unstructured.GetName())
	return &unstructuredFromServer
}

func (r *KymaReconciler) CreateModule(ctx context.Context, name string, kyma *v1alpha1.Kyma,
	module *parsed.Module,
) error {
	if err := r.setupModule(module, kyma, name); err != nil {
		return err
	}
	// create resource if not found
	if err := r.Client.Create(ctx, module.Unstructured, &client.CreateOptions{}); err != nil {
		return fmt.Errorf("error creating custom resource of type %s %w", name, err)
	}

	return nil
}

func (r *KymaReconciler) UpdateModule(ctx context.Context, name string, kyma *v1alpha1.Kyma,
	module *parsed.Module,
) error {
	if module.Template.Spec.Target == v1alpha1.TargetControlPlane {
		return nil
	}

	if err := r.setupModule(module, kyma, name); err != nil {
		return err
	}

	if err := r.Update(ctx, module.Unstructured, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("error updating custom resource of type %s %w", name, err)
	}

	return nil
}

func (r *KymaReconciler) setupModule(module *parsed.Module, kyma *v1alpha1.Kyma, name string) error {
	// set labels
	module.ApplyLabels(kyma, name)

	if module.GetOwnerReferences() == nil {
		// set owner reference
		if err := controllerutil.SetControllerReference(kyma, module.Unstructured, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				name, kyma.Name, err)
		}
	}

	return nil
}

func (r *KymaReconciler) TriggerKymaDeletion(ctx context.Context, kyma *v1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	namespacedName := types.NamespacedName{
		Namespace: kyma.GetNamespace(),
		Name:      kyma.GetName(),
	}.String()
	if kyma.Spec.Sync.Enabled {
		if err := remote.DeleteRemotelySyncedKyma(ctx, r.Client, kyma); client.IgnoreNotFound(err) != nil {
			logger.Info(namespacedName + " could not be deleted remotely!")
			return fmt.Errorf("error occurred while trying to delete remotely synced kyma: %w", err)
		}
		logger.Info(namespacedName + " got deleted remotely!")
	}
	return nil
}

func (r *KymaReconciler) UpdateStatus(
	ctx context.Context, kyma *v1alpha1.Kyma, state v1alpha1.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma,
		state, "templates could not be fetched"); err != nil {
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	r.Event(kyma, "Normal", "StatusUpdate", message)
	return nil
}

func (r *KymaReconciler) UpdateStatusFromErr(
	ctx context.Context, kyma *v1alpha1.Kyma, state v1alpha1.State, err error,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma,
		state, err.Error()); err != nil {
		return fmt.Errorf("error while updating status to %s: %w", state, err)
	}
	r.Event(kyma, "Warning", "StatusUpdate", err.Error())
	return nil
}

func (r *KymaReconciler) GetModules(ctx context.Context, kyma *v1alpha1.Kyma) (parsed.Modules, error) {
	// fetch templates
	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		return nil, fmt.Errorf("templates could not be fetched: %w", err)
	}

	verification, err := r.NewVerification(ctx, kyma.GetNamespace())
	if err != nil {
		return nil, err
	}

	// these are the actual modules
	modules, err := parsed.TemplatesToModules(kyma, templates,
		&parsed.ModuleConversionSettings{Verification: verification})
	if err != nil {
		return nil, fmt.Errorf("could not convert templates to modules: %w", err)
	}

	return modules, nil
}

func (r *KymaReconciler) UpdateStatusModuleInfos(ctx context.Context, kyma *v1alpha1.Kyma) bool {
	requireUpdateCondition := false
	moduleInfoMap := kyma.GetModuleInfoMap()
	moduleInfos := kyma.GetNoLongerExistingModuleInfos()
	if len(moduleInfos) == 0 {
		return false
	}
	for i := range moduleInfos {
		moduleInfo := moduleInfos[i]
		module := unstructured.Unstructured{}
		module.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   moduleInfo.GroupVersionKind.Group,
			Version: moduleInfo.GroupVersionKind.Version,
			Kind:    moduleInfo.GroupVersionKind.Kind,
		})
		err := r.getModule(ctx, &module)
		if k8serrors.IsNotFound(err) {
			requireUpdateCondition = true
			delete(moduleInfoMap, moduleInfo.ModuleName)
		}
	}
	kyma.Status.ModuleInfos = convertToNewModuleInfos(moduleInfoMap)
	return requireUpdateCondition
}

func (r *KymaReconciler) DeleteNoLongerExistingModules(ctx context.Context, moduleInfos []*v1alpha1.ModuleInfo) error {
	var err error
	if len(moduleInfos) == 0 {
		return nil
	}
	for i := range moduleInfos {
		moduleInfo := moduleInfos[i]
		err = r.deleteModule(ctx, moduleInfo)
	}

	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("error deleting module %w", err)
	}
	return nil
}

func convertToNewModuleInfos(moduleInfoMap map[string]*v1alpha1.ModuleInfo) []v1alpha1.ModuleInfo {
	newModuleInfos := make([]v1alpha1.ModuleInfo, 0)
	for _, moduleInfo := range moduleInfoMap {
		newModuleInfos = append(newModuleInfos, *moduleInfo)
	}
	return newModuleInfos
}

func (r *KymaReconciler) deleteModule(ctx context.Context, moduleInfo *v1alpha1.ModuleInfo) error {
	module := unstructured.Unstructured{}
	module.SetNamespace(moduleInfo.Namespace)
	module.SetName(moduleInfo.Name)
	module.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   moduleInfo.GroupVersionKind.Group,
		Version: moduleInfo.GroupVersionKind.Version,
		Kind:    moduleInfo.GroupVersionKind.Kind,
	})
	return r.Delete(ctx, &module, &client.DeleteOptions{})
}

func (r *KymaReconciler) getModule(ctx context.Context, module *unstructured.Unstructured) error {
	return r.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
}
