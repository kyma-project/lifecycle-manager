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
	"github.com/kyma-project/kyma-operator/operator/pkg/remote"
	v1 "k8s.io/api/core/v1"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"strings"
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
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
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.updateKymaStatus(ctx, &kyma, operatorv1alpha1.KymaStateDeleting, "deletion timestamp set")
	}

	// check finalizer
	if !controllerutil.ContainsFinalizer(&kyma, labels.Finalizer) {
		controllerutil.AddFinalizer(&kyma, labels.Finalizer)
		return ctrl.Result{}, r.Update(ctx, &kyma)
	}

	remoteClient, err := r.InitializeRemoteClient(ctx, &kyma)
	if err != nil {
		return ctrl.Result{}, err
	}
	remoteKyma, err := r.FetchRemoteKyma(ctx, remoteClient, &kyma)
	if err != nil {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}
	if synchronizationRequiresRequeue, err := r.SynchronizeRemoteKyma(ctx, remoteClient, &kyma, remoteKyma); err != nil || synchronizationRequiresRequeue {
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	// state handling
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateProcessing:
		return ctrl.Result{}, r.HandleProcessingState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateDeleting:
		if dependentsDeleting, err := r.HandleDeletingState(ctx, &logger, &kyma); err != nil {
			return ctrl.Result{}, err
		} else if dependentsDeleting {
			return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
		}
	case operatorv1alpha1.KymaStateError:
		return ctrl.Result{RequeueAfter: 3 * time.Second}, r.HandleErrorState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateReady:
		//TODO Adjust again
		return ctrl.Result{RequeueAfter: 20 * time.Second}, r.HandleReadyState(ctx, &logger, &kyma)
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
	if err := r.reconcileKymaForRelease(ctx, kyma, templates); err != nil {
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
		r.Recorder.Event(kyma, "Normal", "ReconciliationSuccess", message)
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
		return r.KymaStatus().UpdateStatus(ctx, kyma, operatorv1alpha1.KymaStateError, err.Error())
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
	return r.KymaStatus().UpdateStatus(ctx, kyma, state, message)
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
			util.SetComponentCRLabels(actualComponentStruct, component.Name, channel)
			// set owner reference
			if err := controllerutil.SetOwnerReference(kymaObj, actualComponentStruct, r.Scheme); err != nil {
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
			condition, exists := r.KymaStatus().GetReadyConditionForComponent(kymaObj, component.Name)
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
				util.SetComponentCRLabels(actualComponentStruct, component.Name, channel)

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

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma, templates release.TemplateLookupResultsByName) error {
	logger := log.FromContext(ctx)
	affectedComponents, err := r.CreateOrUpdateComponentsFromTemplate(ctx, kyma, templates)

	if err != nil {
		message := fmt.Sprintf("Component CR creation error: %s", err.Error())
		logger.Info(message)
		r.Recorder.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))
		statusErr := r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message)
		if statusErr != nil {
			return statusErr
		}
		return err
	}

	if len(affectedComponents) > 0 {
		// check component conditions, if not present add them
		logger.Info("checking condition for component CRs")
		r.KymaStatus().AddReadyConditionForObjects(kyma, affectedComponents, operatorv1alpha1.ConditionStatusFalse, "initial condition for component CR")
		release.New(kyma.Status.ActiveChannel, kyma.Spec.Channel, r.KymaStatus().GetEventAdapter(kyma)).IssueChannelChangeInProgress()
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

		updated, err := r.KymaStatus().UpdateComponentConditions(actualComponentStruct, kyma)
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
func (r *KymaReconciler) SetupWithManager(setupLog logr.Logger, mgr ctrl.Manager) error {
	c, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	informers := dynamicinformer.NewDynamicSharedInformerFactory(c, time.Minute*30)
	err = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		informers.Start(ctx.Done())
		return nil
	}))
	if err != nil {
		return err
	}

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).For(&operatorv1alpha1.Kyma{})

	//TODO maybe replace with native REST Handling
	cs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	// This fetches all resources for our component operator CRDs, might become a problem if component operators
	// create their own CRDs that we dont need to watch
	gv := schema.GroupVersion{
		Group:   labels.ComponentPrefix,
		Version: "v1alpha1",
	}
	resources, err := cs.ServerResourcesForGroupVersion(gv.String())
	if client.IgnoreNotFound(err) != nil {
		return err
	}

	// resources found
	if err == nil {
		dynamicInformerSet := make(map[string]*source.Informer)
		for _, resource := range resources.APIResources {
			//TODO Verify if this filtering is really necessary or if we can somehow only listen to status changes instead of resource changes with ResourceVersionChangedPredicate
			if strings.HasSuffix(resource.Name, "status") {
				continue
			}
			gvr := gv.WithResource(resource.Name)
			dynamicInformerSet[gvr.String()] = &source.Informer{Informer: informers.ForResource(gvr).Informer()}
		}

		for gvr, informer := range dynamicInformerSet {
			controllerBuilder = controllerBuilder.
				Watches(informer, &handler.Funcs{UpdateFunc: r.ComponentChangeHandler().ComponentChange(context.TODO())},
					builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
			setupLog.Info("initialized dynamic watching", "source", gvr)
		}
	}

	controllerBuilder = controllerBuilder.Watches(
		&source.Kind{Type: &operatorv1alpha1.ModuleTemplate{}},
		handler.EnqueueRequestsFromMapFunc(r.TemplateChangeHandler().Watch(context.TODO())),
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))

	// here we define a watch on secrets for the kyma operator so that the cache is picking up changes
	controllerBuilder = controllerBuilder.Watches(&source.Kind{Type: &v1.Secret{}}, handler.Funcs{})

	if err := index.TemplateChannel().With(context.TODO(), mgr.GetFieldIndexer()); err != nil {
		return err
	}

	return controllerBuilder.Complete(r)
}

func (r *KymaReconciler) ComponentChangeHandler() *watch.ComponentChangeHandler {
	return &watch.ComponentChangeHandler{Reader: r.Client, StatusWriter: r.Status(), EventRecorder: r.Recorder}
}

func (r *KymaReconciler) TemplateChangeHandler() *watch.TemplateChangeHandler {
	return &watch.TemplateChangeHandler{Reader: r.Client, StatusWriter: r.Status(), EventRecorder: r.Recorder}
}

func (r *KymaReconciler) KymaStatus() *status.Kyma {
	return &status.Kyma{StatusWriter: r.Status(), EventRecorder: r.Recorder}
}

func (r *KymaReconciler) InitializeRemoteClient(ctx context.Context, kyma *operatorv1alpha1.Kyma) (client.Client, error) {
	cc := remote.ClusterClient{DefaultClient: r.Client}

	rc, err := cc.GetRestConfigFromSecret(ctx, kyma.GetName(), kyma.GetNamespace())
	if err != nil {
		r.Recorder.Event(kyma, "Warning", err.Error(), "Rest Config could not be fetched")
		return nil, err
	}

	remoteClient, err := cc.GetNewClient(rc, client.Options{Scheme: r.Scheme})
	if err != nil {
		r.Recorder.Event(kyma, "Warning", err.Error(), "Remote Client could not be created")
		return nil, err
	}

	return remoteClient, nil
}

func (r *KymaReconciler) FetchRemoteKyma(ctx context.Context, remoteClient client.Client, kyma *operatorv1alpha1.Kyma) (*operatorv1alpha1.Kyma, error) {
	remoteKyma := &operatorv1alpha1.Kyma{}
	err := remoteClient.Get(ctx, client.ObjectKeyFromObject(kyma), remoteKyma)

	if meta.IsNoMatchError(err) {
		r.Recorder.Event(kyma, "Normal", err.Error(), "CRDs are missing in SKR and will be installed")
		crd := v1extensions.CustomResourceDefinition{}
		err = r.Get(ctx, client.ObjectKey{Name: fmt.Sprintf("%s.%s", "kymas", operatorv1alpha1.GroupVersion.Group)}, &crd)
		if err != nil {
			return nil, err
		}
		remoteCrd := v1extensions.CustomResourceDefinition{}
		remoteCrd.Name = crd.Name
		remoteCrd.Namespace = crd.Namespace
		remoteCrd.Spec = crd.Spec
		err = remoteClient.Create(ctx, &remoteCrd)
		if err != nil {
			return nil, err
		}
		r.Recorder.Event(kyma, "Normal", "CRDInstallation", "CRDs were installed to SKR")
		return nil, err
	}

	if errors.IsNotFound(err) {
		remoteKyma.Name = kyma.Name
		remoteKyma.Namespace = kyma.Namespace
		remoteKyma.Spec = *kyma.Spec.DeepCopy()
		err = remoteClient.Create(ctx, remoteKyma)
		if err != nil {
			r.Recorder.Event(remoteKyma, "Warning", err.Error(), "Client could not create remote Kyma")
			return nil, err
		}

		return remoteKyma, r.Status().Update(ctx, kyma)
	} else if err != nil {
		r.Recorder.Event(kyma, "Warning", err.Error(), "Client could not fetch remote Kyma")
		return nil, err
	}

	return remoteKyma, err
}

func (r *KymaReconciler) SynchronizeRemoteKyma(ctx context.Context, remoteClient client.Client, kyma *operatorv1alpha1.Kyma, remoteKyma *operatorv1alpha1.Kyma) (bool, error) {
	// check finalizer
	if !controllerutil.ContainsFinalizer(remoteKyma, labels.Finalizer) {
		controllerutil.AddFinalizer(remoteKyma, labels.Finalizer)
	}

	if remoteKyma.Status.ObservedRemoteGeneration != remoteKyma.GetGeneration() {
		kyma.Spec = remoteKyma.Spec
		err := r.Update(ctx, kyma)
		if err != nil {
			r.Recorder.Event(remoteKyma, "Warning", err.Error(), "Client could not update Control Plane Kyma")
		}
		remoteKyma.Status.ObservedRemoteGeneration = remoteKyma.GetGeneration()
		err = remoteClient.Status().Update(ctx, remoteKyma)
		if err != nil {
			return true, err
		}
	}

	if remoteKyma.Status.State != kyma.Status.State {
		remoteKyma.Status.State = kyma.Status.State
		err := remoteClient.Status().Update(ctx, remoteKyma)
		if err != nil {
			return true, err
		}
	}

	lastSyncDate := time.Now().Format(time.RFC3339)
	if remoteKyma.Annotations == nil {
		remoteKyma.Annotations = make(map[string]string)
	}
	remoteKyma.Annotations[labels.LastSync] = lastSyncDate
	err := remoteClient.Update(ctx, remoteKyma)
	if err != nil {
		return true, err
	}

	return false, nil
}
