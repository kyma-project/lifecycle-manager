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
	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"github.com/kyma-project/kyma-operator/operator/pkg/status"
	"github.com/kyma-project/kyma-operator/operator/pkg/util"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"strings"
	"time"
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
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;onEvent;watch

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
	kyma = *kyma.DeepCopy()

	// check if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() && kyma.Status.State != operatorv1alpha1.KymaStateDeleting {
		// if the status is not yet set to deleting, also update the status
		return ctrl.Result{}, r.updateKymaStatus(ctx, &kyma, operatorv1alpha1.KymaStateDeleting, "deletion timestamp set")
	}

	templates := release.GetTemplates(r, ctx, &kyma)
	if kyma.Status.TemplateConfigStatus == operatorv1alpha1.TemplateConfigStatusSynced && util.AreTemplatesOutdated(&logger, &kyma, templates) {
		return ctrl.Result{}, r.HandleTemplateOutdated(ctx, &logger, &kyma)
	}

	// state handling
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateProcessing:
		return ctrl.Result{}, r.HandleProcessingState(ctx, &logger, &kyma, templates)
	case operatorv1alpha1.KymaStateDeleting:
		return ctrl.Result{}, r.HandleDeletingState(ctx)
	case operatorv1alpha1.KymaStateError:
		return ctrl.Result{}, r.HandleErrorState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateReady:
		return ctrl.Result{}, r.HandleReadyState(ctx, &logger, &kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) HandleTemplateOutdated(ctx context.Context, _ *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return r.updateKymaStatus(ctx, kyma.SetTemplateConfigStatusOutdated(), operatorv1alpha1.KymaStateProcessing, "template update")
}

func (r *KymaReconciler) HandleInitialState(ctx context.Context, _ *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateProcessing, "initial state")
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma, templates release.TemplatesByName) error {
	logger.Info("processing " + kyma.Name)

	if err := r.reconcileKymaForRelease(ctx, kyma, templates); err != nil {
		return err
	}

	if kyma.AreAllReadyConditionsSetForKyma() {
		message := fmt.Sprintf("reconciliation of %s finished!", kyma.Name)
		logger.Info(message)
		r.Recorder.Event(kyma, "Normal", "ReconciliationSuccess", message)
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateReady, message)
	}

	return nil
}

func (r *KymaReconciler) HandleDeletingState(_ context.Context) error {
	return nil
}

func (r *KymaReconciler) HandleErrorState(_ context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	if kyma.Status.ObservedGeneration == kyma.Generation {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
	}
	return nil
}

func (r *KymaReconciler) HandleReadyState(_ context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	if kyma.Status.ObservedGeneration == kyma.Generation {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
	}
	return nil
}

func (r *KymaReconciler) updateKyma(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return r.Update(ctx, kyma)
}

func (r *KymaReconciler) updateKymaStatus(ctx context.Context, kyma *operatorv1alpha1.Kyma, state operatorv1alpha1.KymaState, message string) error {
	return r.KymaStatus().UpdateStatus(ctx, kyma, state, message)
}

func (r *KymaReconciler) CreateOrUpdateComponentsFromConfigMap(ctx context.Context, kymaObj *operatorv1alpha1.Kyma, templates release.TemplatesByName) ([]util.ComponentsAssociatedWithTemplate, error) {
	kymaObjectKey := client.ObjectKey{Name: kymaObj.Name, Namespace: kymaObj.Namespace}
	namespacedName := kymaObjectKey.String()
	logger := log.FromContext(ctx).WithName(namespacedName)
	channel := kymaObj.Spec.Channel

	if len(kymaObj.Spec.Components) < 1 {
		return nil, fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	var componentNamesAffected []util.ComponentsAssociatedWithTemplate
	for _, component := range kymaObj.Spec.Components {
		componentName := component.Name + "-name"

		configMap, configMapPresent := templates[component.Name]
		if !configMapPresent || configMap == nil {
			err := fmt.Errorf("could not find template configmap for resource %s and release %s, will not re-queue resource %s", component.Name, channel, namespacedName)
			logger.Error(err, "config map lookup failed")
			return nil, err
		}

		gvk, spec, err := util.GetGvkAndSpecFromConfigMap(configMap, component.Name)
		if err != nil {
			return nil, err
		}
		// combine config map and Kyma settings for component
		if err = mergo.Merge(spec, component.Settings); err != nil {
			return nil, err
		}

		res := unstructured.Unstructured{}
		res.SetGroupVersionKind(*gvk)

		err = r.Get(ctx, client.ObjectKey{Namespace: kymaObj.Namespace, Name: componentName}, &res)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}

		configMapHash := util.AsHash(configMap.Data)

		// overwrite labels for upgrade / downgrade of component versions
		if errors.IsNotFound(err) {
			componentUnstructured := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind":       gvk.Kind,
					"apiVersion": gvk.Group + "/" + gvk.Version,
					"metadata": map[string]interface{}{
						"name":      componentName,
						"namespace": kymaObj.Namespace,
						"labels":    map[string]interface{}{},
					},
					"spec": spec,
				},
			}

			// set labels
			util.SetComponentCRLabels(componentUnstructured, component.Name, channel)

			// set owner reference
			if err := controllerutil.SetOwnerReference(kymaObj, componentUnstructured, r.Scheme); err != nil {
				return nil, fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			// create resource if not found
			if err := r.Client.Create(ctx, componentUnstructured, &client.CreateOptions{}); err != nil {
				return nil, fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully created component CR of", "type", component.Name)

			componentNamesAffected = append(componentNamesAffected, util.ComponentsAssociatedWithTemplate{
				ComponentName: component.Name,
				TemplateHash:  configMapHash,
			})
		} else if kymaObj.Status.TemplateConfigStatus == operatorv1alpha1.TemplateConfigStatusOutdated {
			for _, condition := range kymaObj.Status.Conditions {
				if condition.Reason == component.Name && condition.TemplateHash != *configMapHash {
					updatedComponent := res.DeepCopy()

					// overwrite spec
					updatedComponent.Object["spec"] = spec

					// set labels
					util.SetComponentCRLabels(updatedComponent, component.Name, channel)

					if err := r.Client.Update(ctx, updatedComponent, &client.UpdateOptions{}); err != nil {
						return nil, fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
					}

					logger.Info("successfully updated component CR of", "type", component.Name)
					componentNamesAffected = append(componentNamesAffected, util.ComponentsAssociatedWithTemplate{
						ComponentName: component.Name,
						TemplateHash:  configMapHash,
					})
				}
			}
		}
	}
	return componentNamesAffected, nil
}

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma, templates release.TemplatesByName) error {
	logger := log.FromContext(ctx)
	affectedComponents, err := r.CreateOrUpdateComponentsFromConfigMap(ctx, kyma, templates)

	if err != nil {
		message := fmt.Sprintf("Component CR creation error: %s", err.Error())
		logger.Info(message)
		r.Recorder.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message)
	}

	if len(affectedComponents) > 0 {
		// check component conditions, if not present add them
		logger.Info("checking condition for component CRs")
		r.KymaStatus().AddReadyConditionForObjects(kyma, affectedComponents, operatorv1alpha1.ConditionStatusFalse, "initial condition for component CR")
		release.New(kyma.Status.ActiveChannel, kyma.Spec.Channel, r.KymaStatus().GetEventAdapter(kyma)).IssueChannelChangeInProgress()
		return r.updateKymaStatus(ctx, kyma.SetTemplateConfigStatusSynced(), kyma.Status.State, "")
	}

	return nil
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
	// This fetches all resources for our component operator CRDs, might become a problem if component operators
	// create their own CRDs that we dont need to watch
	gv := schema.GroupVersion{
		Group:   labels.ComponentPrefix,
		Version: "v1alpha1",
	}
	resources, err := cs.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return err
	}

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

	registration := watch.NewKymaChannelWatch(mgr)
	//Add the channel as field watch
	if err := registration.Index(context.TODO()); err != nil {
		return err
	}

	controllerBuilder = controllerBuilder.Watches(
		&source.Kind{Type: &corev1.ConfigMap{}},
		handler.EnqueueRequestsFromMapFunc(registration.Watch(context.TODO())),
		builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
	)

	return controllerBuilder.Complete(r)
}

func (r *KymaReconciler) ComponentChangeHandler() *watch.ComponentChangeHandler {
	return &watch.ComponentChangeHandler{Reader: r.Client, StatusWriter: r.Status(), EventRecorder: r.Recorder}
}

func (r *KymaReconciler) KymaStatus() *status.Kyma {
	return &status.Kyma{StatusWriter: r.Status(), EventRecorder: r.Recorder}
}
