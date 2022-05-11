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
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *KymaReconciler) GetEventAdapter(kyma *operatorv1alpha1.Kyma) adapter.Eventing {
	return func(eventtype, reason, message string) {
		r.Recorder.Event(kyma, eventtype, reason, message)
	}
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

	// state handling
	switch kyma.Status.State {
	case "":
		return ctrl.Result{}, r.HandleInitialState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateProcessing:
		return ctrl.Result{}, r.HandleProcessingState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateDeleting:
		return ctrl.Result{}, r.HandleDeletingState(ctx)
	case operatorv1alpha1.KymaStateError:
		return ctrl.Result{}, r.HandleErrorState(ctx, &logger, &kyma)
	case operatorv1alpha1.KymaStateReady:
		return ctrl.Result{}, r.HandleReadyState(ctx, &logger, &kyma)
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) HandleInitialState(_ context.Context, _ *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return r.updateKymaStatus(context.TODO(), kyma, operatorv1alpha1.KymaStateProcessing, "initial state")
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	logger.Info("processing " + kyma.Name)

	if err := r.reconcileKymaForRelease(ctx, kyma); err != nil {
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
	kyma.Status.State = state
	switch state {
	case operatorv1alpha1.KymaStateReady:
		addReadyConditionForObjects(kyma, []string{operatorv1alpha1.KymaKind}, operatorv1alpha1.ConditionStatusTrue, message)
		// set active release only when ready state is set
		kyma.SetActiveRelease()
	case "":
		addReadyConditionForObjects(kyma, []string{operatorv1alpha1.KymaKind}, operatorv1alpha1.ConditionStatusUnknown, message)
	default:
		addReadyConditionForObjects(kyma, []string{operatorv1alpha1.KymaKind}, operatorv1alpha1.ConditionStatusFalse, message)
	}
	return r.Status().Update(ctx, kyma.SetObservedGeneration())
}

func (r *KymaReconciler) GetTemplateConfigMapForRelease(ctx context.Context, component, release string) (*corev1.ConfigMap, error) {
	configMapList := &corev1.ConfigMapList{}
	if err := r.List(ctx, configMapList,
		client.MatchingLabels{
			labels.ControllerName: component,
			labels.Release:        release,
		},
	); err != nil {
		return nil, err
	}

	if len(configMapList.Items) > 1 {
		return nil, fmt.Errorf("more than one config map template found for component: %s", component)
	}

	if len(configMapList.Items) == 0 {
		if err := r.List(ctx, configMapList,
			client.MatchingLabels{
				labels.ControllerName: component,
			},
		); err != nil {
			return nil, err
		}

		if len(configMapList.Items) > 1 {
			return nil, fmt.Errorf("more than one config map template found for component: %s", component)
		}

		if len(configMapList.Items) == 0 {
			return nil, fmt.Errorf("no config map template found for component: %s", component)
		}

	}

	actualReleaseVersion := configMapList.Items[0].GetLabels()[labels.Release]
	if actualReleaseVersion == "" {
		actualReleaseVersion = "unversioned"
	}

	if actualReleaseVersion != release {
		log.FromContext(ctx).Info(fmt.Sprintf("using %s (instead of %s) for component %s", actualReleaseVersion, release, component))
	} else {
		log.FromContext(ctx).Info(fmt.Sprintf("using %s for component %s", actualReleaseVersion, component))
	}

	return &configMapList.Items[0], nil
}

func (r *KymaReconciler) CreateComponentsFromConfigMap(ctx context.Context, kymaObj *operatorv1alpha1.Kyma, release release.Release) ([]string, error) {
	kymaObjectKey := client.ObjectKey{Name: kymaObj.Name, Namespace: kymaObj.Namespace}
	namespacedName := kymaObjectKey.String()
	logger := log.FromContext(ctx).WithName(namespacedName)

	if len(kymaObj.Spec.Components) < 1 {
		return nil, fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	var componentNamesCreated []string
	for _, component := range kymaObj.Spec.Components {
		componentName := component.Name + "-name"

		configMap, err := r.GetTemplateConfigMapForRelease(ctx, component.Name, release.GetNew())
		if err != nil {
			logger.Error(err, fmt.Sprintf("could not find template configmap for resource %s and release %s, will not re-queue resource %s", component.Name, release.GetNew(), namespacedName))
			return nil, err
		}
		gvk, spec, err := getGvkAndSpecFromConfigMap(configMap, component.Name)
		if err != nil {
			return nil, err
		}
		res := unstructured.Unstructured{}
		res.SetGroupVersionKind(*gvk)

		err = r.Get(ctx, client.ObjectKey{Namespace: kymaObj.Namespace, Name: componentName}, &res)
		if client.IgnoreNotFound(err) != nil {
			return nil, err
		}

		// overwrite labels for upgrade / downgrade of component versions
		// KymaUpdate doesn't require an update
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
			for key, value := range component.Settings {
				componentUnstructured.Object["spec"].(map[string]interface{})[key] = value
			}

			// set labels
			setComponentCRLabels(componentUnstructured, component.Name, release)

			// set owner reference
			if err := controllerutil.SetOwnerReference(kymaObj, componentUnstructured, r.Scheme); err != nil {
				return nil, fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			// create resource if not found
			if err := r.Client.Create(ctx, componentUnstructured, &client.CreateOptions{}); err != nil {
				return nil, fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully created component CR of", "type", component.Name)

			componentNamesCreated = append(componentNamesCreated, component.Name)
		}
	}
	return componentNamesCreated, nil
}

func (r *KymaReconciler) UpdateProgressionLabelsForComponentCRs(ctx context.Context, kymaObj *operatorv1alpha1.Kyma, release release.Release) error {
	namespacedName := client.ObjectKey{Name: kymaObj.Name, Namespace: kymaObj.Namespace}.String()
	logger := log.FromContext(ctx).WithName(namespacedName)
	// get all component CRs by label
	for _, component := range kymaObj.Spec.Components {
		componentName := component.Name + "-name"
		configMap, err := r.GetTemplateConfigMapForRelease(ctx, component.Name, release.GetNew())
		if err != nil {
			logger.Error(err, fmt.Sprintf("could not find template configmap for resource %s and release %s, will not re-queue resource %s", component.Name, release.GetNew(), namespacedName))
			return err
		}
		gvk, _, err := getGvkAndSpecFromConfigMap(configMap, component.Name)
		if err != nil {
			return err
		}
		res := unstructured.Unstructured{}
		res.SetGroupVersionKind(*gvk)

		if err = r.Get(ctx, client.ObjectKey{Namespace: kymaObj.Namespace, Name: componentName}, &res); err != nil {
			return err
		}

		// set labels
		setComponentCRLabels(&res, component.Name, release)

		if err := r.Client.Update(ctx, &res); err != nil {
			return fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
		}

	}
	return nil
}

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)

	rel := release.New(kyma.Status.ActiveRelease, kyma.Spec.Release, r.GetEventAdapter(kyma))

	componentNamesCreated, err := r.CreateComponentsFromConfigMap(ctx, kyma, rel)

	if len(componentNamesCreated) > 0 {
		// check component conditions, if not present add them
		logger.Info("checking condition for component CRs")
		addReadyConditionForObjects(kyma, componentNamesCreated, operatorv1alpha1.ConditionStatusFalse, "initial condition for component CR")
		return r.updateKymaStatus(ctx, kyma, kyma.Status.State, "")
	}

	// no update required for KymaUpdate
	if err == nil && rel.GetType() != release.Update {
		err = r.UpdateProgressionLabelsForComponentCRs(ctx, kyma, rel)
	}

	rel.IssueReleaseEvent()

	if err != nil {
		message := fmt.Sprintf("Component CR creation error: %s", err.Error())
		logger.Info(message)
		r.Recorder.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager) error {
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

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Kyma{}).
		Watches(
			&source.Informer{Informer: informers.ForResource(schema.GroupVersionResource{
				//TODO: hard-coded, remove!
				Group:    "component.kyma-project.io",
				Version:  "v1alpha1",
				Resource: "manifests",
			}).Informer()},
			&handler.Funcs{
				UpdateFunc: r.ComponentChangeHandler,
			}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *KymaReconciler) ComponentChangeHandler(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	objectBytes, err := json.Marshal(e.ObjectNew)
	if err != nil {
		return
	}
	componentObj := unstructured.Unstructured{}
	if err = json.Unmarshal(objectBytes, &componentObj); err != nil {
		return
	}
	if componentObj.Object["status"] == nil {
		return
	}
	for key, value := range componentObj.Object["status"].(map[string]interface{}) {
		if key == "state" && value == string(operatorv1alpha1.KymaStateReady) {
			ownerRefs := componentObj.GetOwnerReferences()
			var ownerName string
			kymaObj := &operatorv1alpha1.Kyma{}
			for _, ownerRef := range ownerRefs {
				if operatorv1alpha1.KymaKind == ownerRef.Kind {
					ownerName = ownerRef.Name
					break
				}
			}

			kymaNamespacedName := client.ObjectKey{Name: ownerName, Namespace: componentObj.GetNamespace()}
			if err := r.Get(context.TODO(), kymaNamespacedName, kymaObj); err != nil {
				return
			}

			componentNameLabel := componentObj.GetLabels()[labels.ControllerName]
			if componentNameLabel == "" {
				return
			}

			// TODO: "istio", "serverless" are hard-coded, remove!
			addReadyConditionForObjects(kymaObj, []string{componentNameLabel, "istio", "serverless"}, operatorv1alpha1.ConditionStatusTrue,
				fmt.Sprintf("successfully installed component : %s", e.ObjectNew.GetObjectKind().GroupVersionKind().String()))

			// triggers reconciliation on Kyma
			if err := r.updateKymaStatus(context.TODO(), kymaObj, kymaObj.Status.State,
				fmt.Sprintf("component %s set to %s state", componentNameLabel, string(operatorv1alpha1.KymaStateReady))); err != nil {
				return
			}
		}
	}
}
