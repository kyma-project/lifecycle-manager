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
	"time"

	v2 "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/apis/v2/signatures"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/dynamic"
	"github.com/kyma-project/kyma-operator/operator/pkg/img"
	"github.com/kyma-project/kyma-operator/operator/pkg/remote"
	"github.com/kyma-project/kyma-operator/operator/pkg/signature"
	errwrap "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"

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

var ErrNoComponentSpecified = errwrap.New("no component specified")

type RequeueIntervals struct {
	Success time.Duration
	Failure time.Duration
	Waiting time.Duration
}

type ModuleVerificationSettings struct {
	EnableVerification  bool
	PublicKeyFilePath   string
	ValidSignatureNames []string
}

// KymaReconciler reconciles a Kyma object.
type KymaReconciler struct {
	client.Client
	record.EventRecorder
	RequeueIntervals
	ModuleVerificationSettings
}

//nolint:lll
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
	logger := log.FromContext(ctx)
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	// check if kyma resource exists
	kyma := &operatorv1alpha1.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")

		return ctrl.Result{}, client.IgnoreNotFound(err) //nolint:wrapcheck
	}

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != operatorv1alpha1.KymaStateDeleting {
		if err := r.TriggerKymaDeletion(ctx, kyma); err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
		}
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, errwrap.Wrap(
			status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateDeleting, "deletion timestamp set"),
			"could not update kyma status after triggering deletion")
	}

	// check finalizer
	if labels.CheckLabelsAndFinalizers(kyma) {
		return ctrl.Result{}, errwrap.Wrap(r.Update(ctx, kyma), "could not update kyma after finalizer check")
	}

	// create a remote synchronization context, and update the remote kyma with the state of the control plane
	if kyma.Spec.Sync.Enabled {
		err := r.updateRemote(ctx, kyma)
		if err != nil {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, err
		}
	}

	// state handling
	return r.stateHandling(ctx, kyma)
}

func (r *KymaReconciler) updateRemote(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	syncContext, err := remote.InitializeKymaSynchronizationContext(ctx, r.Client, kyma)
	if err != nil {
		return fmt.Errorf("could not initialize remote context before updating remote kyma: %w", err)
	}
	remoteKyma, err := syncContext.CreateOrFetchRemoteKyma(ctx)
	if err != nil {
		return fmt.Errorf("could not fetch kyma updating remote kyma: %w", err)
	}
	synchronizationRequiresRequeue, err := syncContext.SynchronizeRemoteKyma(ctx, remoteKyma)
	if err != nil || synchronizationRequiresRequeue {
		return fmt.Errorf("could not synchronize remote kyma: %w", err)
	}
	return nil
}

func (r *KymaReconciler) stateHandling(ctx context.Context, kyma *operatorv1alpha1.Kyma) (ctrl.Result, error) {
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, kyma)
	case operatorv1alpha1.KymaStateProcessing:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Failure}, r.HandleProcessingState(ctx, kyma)
	case operatorv1alpha1.KymaStateDeleting:
		if dependentsDeleting, err := r.HandleDeletingState(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{RequeueAfter: r.RequeueIntervals.Waiting}, nil
		}
	case operatorv1alpha1.KymaStateError:
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Waiting}, r.HandleErrorState(ctx, kyma)
	case operatorv1alpha1.KymaStateReady:
		// TODO Adjust again
		return ctrl.Result{RequeueAfter: r.RequeueIntervals.Success}, r.HandleReadyState(ctx, kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) HandleInitialState(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing, "initial state")
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	logger.Info("processing " + kyma.Name)

	if len(kyma.Spec.Components) < 1 {
		return fmt.Errorf("error parsing %s: %w", kyma.Name, ErrNoComponentSpecified)
	}

	// fetch templates
	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateError, "templates could not be fetched")
	}

	verifier, err := r.NewSignatureVerifier(ctx, kyma.GetNamespace())
	if err != nil {
		return err
	}

	// these are the actual modules
	modules, err := util.ParseTemplates(kyma, templates, verifier)
	if err != nil {
		return err
	}

	statusUpdateRequiredFromCreation, err := r.CreateOrUpdateModules(ctx, kyma, modules)
	if err != nil {
		message := fmt.Sprintf("Component CR creation error: %s", err.Error())
		logger.Info(message)
		r.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))

		if err := status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message); err != nil {
			return err
		}

		return err
	}

	// Now we track the conditions: update the status based on their state
	statusUpdateRequiredFromSync, err := r.SyncConditionsWithModuleStates(ctx, kyma, modules)
	if err != nil {
		return err
	}

	// set ready condition if applicable
	if kyma.AreAllReadyConditionsSetForKyma() {
		message := fmt.Sprintf("reconciliation of %s finished!", kyma.Name)
		logger.Info(message)
		r.Event(kyma, "Normal", "ReconciliationSuccess", message)

		return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateReady, message)
	}

	// if the ready condition is not applicable, but we changed the conditions, we still need to issue an update
	if statusUpdateRequiredFromCreation || statusUpdateRequiredFromSync {
		return status.Helper(r).UpdateStatus(ctx, kyma, kyma.Status.State, "updating component conditions")
	}

	return nil
}

func (r *KymaReconciler) HandleDeletingState(ctx context.Context, kyma *operatorv1alpha1.Kyma) (bool, error) {
	logger := log.FromContext(ctx)

	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		return false, status.Helper(r).UpdateStatus(ctx, kyma,
			operatorv1alpha1.KymaStateError, "templates could not be fetched")
	}

	verifier, err := r.NewSignatureVerifier(ctx, kyma.GetNamespace())
	if err != nil {
		return false, err
	}

	modules, err := util.ParseTemplates(kyma, templates, verifier)
	if err != nil {
		return false, err
	}

	for name, module := range modules {
		if err = r.Get(ctx, client.ObjectKeyFromObject(module.Unstructured), module.Unstructured); err == nil {
			// component CR still exists
			logger.Info(fmt.Sprintf("deletion cannot proceed - waiting for component CR %s to be deleted for %s",
				name, client.ObjectKeyFromObject(kyma)))

			return true, nil
		} else if !errors.IsNotFound(err) {
			// unknown error while getting component CR
			return false, fmt.Errorf("deletion cannot proceed - unknown error: %w", err)
		}
	}

	// remove finalizer
	logger.Info("All component CRs have been removed, removing finalizer",
		"resource", client.ObjectKeyFromObject(kyma))

	if kyma.Spec.Sync.Enabled {
		if err := remote.RemoveFinalizerFromRemoteKyma(ctx, r, kyma); client.IgnoreNotFound(err) != nil {
			return false, errwrap.Wrap(err, "error while trying to remove finalizer from remote")
		}
		logger.Info("removed remote finalizer",
			"resource", client.ObjectKeyFromObject(kyma))
	}

	controllerutil.RemoveFinalizer(kyma, labels.Finalizer)

	return false, errwrap.Wrap(r.Update(ctx, kyma), "error while trying to udpate kyma during deletion")
}

func (r *KymaReconciler) HandleErrorState(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return r.HandleConsistencyChanges(ctx, kyma)
}

func (r *KymaReconciler) HandleReadyState(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return r.HandleConsistencyChanges(ctx, kyma)
}

func (r *KymaReconciler) HandleConsistencyChanges(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	// outdated template
	templates, err := release.GetTemplates(ctx, r, kyma)
	if err != nil {
		logger.Error(err, "error fetching fetching templates")

		return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateError, err.Error())
	}

	for _, template := range templates {
		if template.Outdated {
			return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing, "template update")
		}
	}

	verifier, err := r.NewSignatureVerifier(ctx, kyma.GetNamespace())
	if err != nil {
		return err
	}

	// condition update on CRs
	modules, err := util.ParseTemplates(kyma, templates, verifier)
	if err != nil {
		return err
	}

	statusUpdateRequired, err := r.SyncConditionsWithModuleStates(ctx, kyma, modules)
	if err != nil {
		logger.Error(err, "error while updating component status conditions")
	}

	// at least one condition changed during the sync
	if statusUpdateRequired {
		return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing,
			"updating component conditions")
	}

	// generation change
	if kyma.Status.ObservedGeneration != kyma.Generation {
		return status.Helper(r).UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing,
			"object updated")
	}

	return nil
}

func (r *KymaReconciler) SyncConditionsWithModuleStates(ctx context.Context, kyma *operatorv1alpha1.Kyma,
	modules util.Modules,
) (bool, error) {
	// Now we track the conditions: update the status based on their state
	statusUpdateRequired := false

	var err error

	// Now, iterate through each module and compare the fitting condition in the Kyma CR to the state of the module
	for name, module := range modules {
		var conditionsUpdated bool
		// Next, we fetch it from the API Server to determine its status
		if err = r.Get(ctx, client.ObjectKeyFromObject(module.Unstructured), module.Unstructured); err != nil {
			break
		}
		// Finally, we update the condition based on its state and remember if we had to do an update
		if conditionsUpdated, err = status.Helper(r).UpdateConditionFromComponentState(name, module, kyma); err != nil {
			break
		}

		if !statusUpdateRequired {
			// if any given condition was updated, we have to make sure to trigger a status update later on
			statusUpdateRequired = conditionsUpdated
		}
	}

	return statusUpdateRequired, errwrap.Wrap(err, "error occurred while synchronizing conditions with states")
}

func (r *KymaReconciler) CreateOrUpdateModules(ctx context.Context, kyma *operatorv1alpha1.Kyma,
	modules util.Modules,
) (bool, error) {
	logger := log.FromContext(ctx).WithName(client.ObjectKey{Name: kyma.Name, Namespace: kyma.Namespace}.String())
	kymaSyncNecessary := false

	for name, module := range modules {
		// either the template in the condition is outdated (reflected by a generation change on the template)
		// or the template that is supposed to be applied changed (e.g. because the kyma spec changed)

		err := r.Get(ctx, client.ObjectKeyFromObject(module.Unstructured), module.Unstructured)
		if client.IgnoreNotFound(err) != nil {
			return false, errwrap.Wrapf(err, "error occurred while fetching module %s", module.GetName())
		}

		if errors.IsNotFound(err) { //nolint:nestif    // create resource if not found
			err := r.CreateModule(ctx, name, kyma, module)
			if err != nil {
				return false, err
			}

			status.Helper(r).SyncReadyConditionForModules(kyma, util.Modules{name: module},
				operatorv1alpha1.ConditionStatusFalse, fmt.Sprintf("initial condition for %s module CR", module.Name))

			logger.Info("successfully created module CR of",
				"type", name,
				"templateChannel", module.Channel(),
				"templateGeneration", module.Template.GetGeneration())

			kymaSyncNecessary = true
		} else if module.TemplateOutdated {
			condition, _ := status.Helper(r).GetReadyConditionForComponent(kyma, name)
			if condition.TemplateInfo.Generation != module.Template.GetGeneration() ||
				condition.TemplateInfo.Channel != module.Template.Spec.Channel {
				if err := r.UpdateModule(ctx, name, kyma, module); err != nil {
					return false, err
				}
				logger.Info("successfully updated component cr",
					"type", name,
					"templateChannel", module.Channel(),
					"templateGeneration", module.Template.GetGeneration())
				status.Helper(r).SyncReadyConditionForModules(kyma, util.Modules{name: module},
					operatorv1alpha1.ConditionStatusFalse, "updated condition for module cr")
				kymaSyncNecessary = true
			}
		}
	}

	return kymaSyncNecessary, nil
}

func (r *KymaReconciler) CreateModule(ctx context.Context, name string, kyma *operatorv1alpha1.Kyma,
	module *util.Module,
) error {
	// merge template and component settings
	if err := util.CopySettingsToUnstructuredFromResource(module.Unstructured, module.Settings); err != nil {
		return errwrap.Wrap(err, "error occurred while creating module from settings")
	}
	// set labels
	util.SetComponentCRLabels(module.Unstructured, name, module.Template.Spec.Channel, kyma.Name)
	// set owner reference - NOT Controller Reference as we use the custom ComponentChangeHandler for watching
	if err := controllerutil.SetOwnerReference(kyma, module.Unstructured, r.Scheme()); err != nil {
		return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
			name, kyma.Name, err)
	}
	// create resource if not found
	if err := r.Client.Create(ctx, module.Unstructured, &client.CreateOptions{}); err != nil {
		return fmt.Errorf("error creating custom resource of type %s %w", name, err)
	}

	return nil
}

func (r *KymaReconciler) UpdateModule(ctx context.Context, name string, kyma *operatorv1alpha1.Kyma,
	module *util.Module,
) error {
	// merge template and component settings
	if err := util.CopySettingsToUnstructuredFromResource(module.Unstructured, module.Settings); err != nil {
		return errwrap.Wrap(err, "error occurred while updating module from settings")
	}
	// set labels
	util.SetComponentCRLabels(module.Unstructured, name, module.Template.Spec.Channel, kyma.Name)
	// update the spec
	module.Unstructured.Object["spec"] = module.Template.Spec.Data.Object["spec"]
	if err := r.Update(ctx, module.Unstructured, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("error updating custom resource of type %s %w", name, err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&operatorv1alpha1.Kyma{}).WithOptions(options).
		Watches(
			&source.Kind{Type: &operatorv1alpha1.ModuleTemplate{}},
			handler.EnqueueRequestsFromMapFunc(watch.NewTemplateChangeHandler(r).Watch(context.TODO())),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		// here we define a watch on secrets for the kyma operator so that the cache is picking up changes
		Watches(&source.Kind{Type: &v1.Secret{}}, handler.Funcs{})

	var dynamicInformers map[string]source.Source

	var err error

	// This fetches all resources for our component operator CRDs, might become a problem if component operators
	// create their own CRDs that we dont need to watch
	gv := schema.GroupVersion{
		Group:   labels.ComponentPrefix,
		Version: "v1alpha1",
	}
	if dynamicInformers, err = dynamic.Informers(mgr, gv); err != nil {
		return errwrap.Wrapf(err, "error while setting up Dynamic Informers for GV %s", gv.String())
	}

	for _, informer := range dynamicInformers {
		controllerBuilder = controllerBuilder.
			Watches(informer, &handler.Funcs{UpdateFunc: watch.NewComponentChangeHandler(r).Watch(context.TODO())},
				builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
	}

	if err := index.TemplateChannel().With(context.TODO(), mgr.GetFieldIndexer()); err != nil {
		return errwrap.Wrap(err, "error while setting up Template Channel Field Indexer")
	}

	return errwrap.Wrap(controllerBuilder.Complete(r), "error occurred while building controller")
}

func (r *KymaReconciler) NewSignatureVerifier(
	ctx context.Context, namespace string,
) (img.SignatureVerification, error) {
	if !r.EnableVerification {
		return img.NoSignatureVerification, nil
	}

	var verifier signatures.Verifier
	var err error
	if r.ModuleVerificationSettings.PublicKeyFilePath == "" {
		verifier, err = signature.CreateRSAVerifierFromSecrets(ctx, r.Client, r.ValidSignatureNames, namespace)
	} else {
		verifier, err = signatures.CreateRSAVerifierFromKeyFile(r.ModuleVerificationSettings.PublicKeyFilePath)
	}
	if err != nil {
		return nil, errwrap.Wrap(err, "error occurred while initializing Signature Verifier")
	}

	return func(descriptor *v2.ComponentDescriptor) error {
		for _, sig := range descriptor.Signatures {
			for _, validName := range r.ModuleVerificationSettings.ValidSignatureNames {
				if sig.Name == validName {
					return errwrap.Wrap(verifier.Verify(*descriptor, sig),
						"error occurred during signature verification")
				}
			}
		}
		return errwrap.New("no signature was found in the descriptor matching the valid signature list")
	}, nil
}

func (r *KymaReconciler) TriggerKymaDeletion(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	namespacedName := types.NamespacedName{
		Namespace: kyma.GetNamespace(),
		Name:      kyma.GetName(),
	}.String()
	if kyma.Spec.Sync.Enabled {
		if err := remote.DeleteRemotelySyncedKyma(ctx, r.Client, kyma); client.IgnoreNotFound(err) != nil {
			logger.Info(namespacedName + " could not be deleted remotely!")
			return errwrap.Wrap(err, "error occurred while trying to delete remotely synced kyma")
		}
		logger.Info(namespacedName + " got deleted remotely!")
	}
	return r.DeleteKymaDependencies(ctx, kyma)
}

// DeleteKymaDependencies takes care of deleting all relevant dependencies of a Kyma Object. To make sure that we really
// catch all Kymas that are available in the given context, we make use of deletion through the label that is set on
// every generated resource. The alternative would be to parse the module templates, however if the module template was
// deleted it could happened that the module were trying to delete can no longer be resolved.
// This is why we take the GVK from the readiness condition and delete all objects of the GVK in the condition.
func (r *KymaReconciler) DeleteKymaDependencies(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	if len(kyma.Status.Conditions) > 0 {
		for _, condition := range kyma.Status.Conditions {
			if condition.Type == operatorv1alpha1.ConditionTypeReady && condition.Reason != operatorv1alpha1.KymaKind {
				gvk := condition.TemplateInfo.GroupVersionKind

				toDelete := &metav1.PartialObjectMetadata{}
				toDelete.SetGroupVersionKind(schema.GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				})

				if err := r.DeleteAllOf(ctx, toDelete, &client.DeleteAllOfOptions{
					ListOptions: client.ListOptions{
						LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{labels.KymaName: kyma.GetName()}),
						Namespace:     kyma.GetNamespace(),
					},
					DeleteOptions: client.DeleteOptions{},
				}); err != nil {
					return errwrap.Wrap(err, "error occurred while trying to delete kyma dependencies")
				}
			}
		}
	}
	return nil
}
