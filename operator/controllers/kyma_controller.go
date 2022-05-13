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
	"crypto/sha256"
	"fmt"
	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/adapter"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

	templates := r.GetTemplates(ctx, &kyma)
	if kyma.Status.TemplateConfigStatus == operatorv1alpha1.TemplateConfigStatusSynced && r.AreTemplatesOutdated(&logger, &kyma, templates) {
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

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma, templates TemplatesByName) error {
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
	kyma.Status.State = state
	switch state {
	case operatorv1alpha1.KymaStateReady:
		addReadyConditionForObjects(kyma, []ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusTrue, message)
		// set active release only when ready state is set
		release.New(kyma.Status.ActiveChannel, kyma.Spec.Channel, r.GetEventAdapter(kyma)).IssueChannelChangeSuccess()
		kyma.SetActiveChannel()
	case "":
		addReadyConditionForObjects(kyma, []ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusUnknown, message)
	default:
		addReadyConditionForObjects(kyma, []ComponentsAssociatedWithTemplate{{
			ComponentName: operatorv1alpha1.KymaKind,
		}}, operatorv1alpha1.ConditionStatusFalse, message)
	}
	return r.Status().Update(ctx, kyma.SetObservedGeneration())
}

func (r *KymaReconciler) CreateOrUpdateComponentsFromConfigMap(ctx context.Context, kymaObj *operatorv1alpha1.Kyma, templates TemplatesByName) ([]ComponentsAssociatedWithTemplate, error) {
	kymaObjectKey := client.ObjectKey{Name: kymaObj.Name, Namespace: kymaObj.Namespace}
	namespacedName := kymaObjectKey.String()
	logger := log.FromContext(ctx).WithName(namespacedName)
	channel := kymaObj.Spec.Channel

	if len(kymaObj.Spec.Components) < 1 {
		return nil, fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	var componentNamesAffected []ComponentsAssociatedWithTemplate
	for _, component := range kymaObj.Spec.Components {
		componentName := component.Name + "-name"

		configMap, configMapPresent := templates[component.Name]
		if !configMapPresent || configMap == nil {
			err := fmt.Errorf("could not find template configmap for resource %s and release %s, will not re-queue resource %s", component.Name, channel, namespacedName)
			logger.Error(err, "config map lookup failed")
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
		var charts []map[string]interface{}
		for _, setting := range component.Settings {
			chart := map[string]interface{}{}
			for key, value := range setting {
				chart[key] = value
			}
			charts = append(charts, chart)
		}
		componentUnstructured.Object["spec"].(map[string]interface{})["charts"] = charts

		// set labels
		setComponentCRLabels(componentUnstructured, component.Name, channel)

		configMapHash := asHash(configMap.Data)

		// overwrite labels for upgrade / downgrade of component versions
		if errors.IsNotFound(err) {
			// set owner reference
			if err := controllerutil.SetOwnerReference(kymaObj, componentUnstructured, r.Scheme); err != nil {
				return nil, fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			// create resource if not found
			if err := r.Client.Create(ctx, componentUnstructured, &client.CreateOptions{}); err != nil {
				return nil, fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully created component CR of", "type", component.Name)

			componentNamesAffected = append(componentNamesAffected, ComponentsAssociatedWithTemplate{
				ComponentName: component.Name,
				TemplateHash:  configMapHash,
			})
		} else if kymaObj.Status.TemplateConfigStatus == operatorv1alpha1.TemplateConfigStatusOutdated {
			for _, condition := range kymaObj.Status.Conditions {
				if condition.Reason == component.Name && condition.TemplateHash != *configMapHash {
					componentUnstructured.SetResourceVersion(res.GetResourceVersion())
					if err := r.Client.Update(ctx, componentUnstructured, &client.UpdateOptions{}); err != nil {
						return nil, fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
					}
					logger.Info("successfully updated component CR of", "type", component.Name)
					componentNamesAffected = append(componentNamesAffected, ComponentsAssociatedWithTemplate{
						ComponentName: component.Name,
						TemplateHash:  configMapHash,
					})
				}
			}
		}
	}
	return componentNamesAffected, nil
}

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma, templates TemplatesByName) error {
	logger := log.FromContext(ctx)

	affectedComponents, err := r.CreateOrUpdateComponentsFromConfigMap(ctx, kyma, templates)

	if len(affectedComponents) > 0 {
		// check component conditions, if not present add them
		logger.Info("checking condition for component CRs")
		addReadyConditionForObjects(kyma, affectedComponents, operatorv1alpha1.ConditionStatusFalse, "initial condition for component CR")
		release.New(kyma.Status.ActiveChannel, kyma.Spec.Channel, r.GetEventAdapter(kyma)).IssueChannelChangeInProgress()
		return r.updateKymaStatus(ctx, kyma.SetTemplateConfigStatusSynced(), kyma.Status.State, "")
	}

	if err != nil {
		message := fmt.Sprintf("Component CR creation error: %s", err.Error())
		logger.Info(message)
		r.Recorder.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message)
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
		if strings.HasSuffix(resource.Name, "status") {
			continue
		}
		gvr := gv.WithResource(resource.Name)
		dynamicInformerSet[gvr.String()] = &source.Informer{Informer: informers.ForResource(gvr).Informer()}
	}

	for gvr, informer := range dynamicInformerSet {
		controllerBuilder = controllerBuilder.
			Watches(informer, &handler.Funcs{UpdateFunc: r.ComponentChangeHandler},
				builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}))
		setupLog.Info("initialized dynamic watching", "source", gvr)
	}

	//Add the channel as field index
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &operatorv1alpha1.Kyma{}, "spec.channel", func(rawObj client.Object) []string {
		kyma := rawObj.(*operatorv1alpha1.Kyma)
		if kyma.Spec.Channel == "" {
			return nil
		}
		return []string{string(kyma.Spec.Channel)}
	}); err != nil {
		return err
	}

	controllerBuilder = controllerBuilder.Watches(
		&source.Kind{Type: &corev1.ConfigMap{}},
		handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			l := object.GetLabels()

			if errors.IsNotFound(r.Get(context.TODO(), client.ObjectKeyFromObject(object), &unstructured.Unstructured{})) {
				return []reconcile.Request{}
			}

			managedBy, managedByPresent := l[labels.ManagedBy]
			controller, controllerLabelPresent := l[labels.ControllerName]
			channel, channelPresent := l[labels.Channel]
			if !controllerLabelPresent || controller == "" || !managedByPresent || managedBy != "kyma-operator" || !channelPresent {
				return []reconcile.Request{}
			}

			affectedKymas := &operatorv1alpha1.KymaList{}

			listOps := &client.ListOptions{
				FieldSelector: fields.OneTermEqualSelector("spec.channel", channel),
			}
			err = r.List(context.TODO(), affectedKymas, listOps)
			if err != nil {
				return []reconcile.Request{}
			}
			requests := make([]reconcile.Request, len(affectedKymas.Items))
			for i, item := range affectedKymas.Items {
				if errors.IsNotFound(r.Get(context.TODO(), client.ObjectKey{
					Namespace: item.Namespace,
					Name:      item.Name,
				}, &unstructured.Unstructured{})) {
					continue
				}
				requests[i] = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      fmt.Sprintf(item.GetName()),
						Namespace: item.GetNamespace(),
					},
				}
			}
			return requests
		}),
		builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
	)

	return controllerBuilder.Complete(r)
}

func (r *KymaReconciler) ComponentChangeHandler(e event.UpdateEvent, _ workqueue.RateLimitingInterface) {
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
			updateReadyCondition(kymaObj, []string{componentNameLabel, "istio", "serverless"}, operatorv1alpha1.ConditionStatusTrue,
				fmt.Sprintf("successfully installed component : %s", e.ObjectNew.GetObjectKind().GroupVersionKind().String()))

			// triggers reconciliation on Kyma
			if err := r.updateKymaStatus(context.TODO(), kymaObj, kymaObj.Status.State,
				fmt.Sprintf("component %s set to %s state", componentNameLabel, string(operatorv1alpha1.KymaStateReady))); err != nil {
				return
			}
		}
	}
}

type TemplatesByName map[string]*corev1.ConfigMap

func (r *KymaReconciler) GetTemplates(ctx context.Context, k *operatorv1alpha1.Kyma) TemplatesByName {
	templates := make(map[string]*corev1.ConfigMap)
	for _, component := range k.Spec.Components {
		configMap, err := release.NewChannelConfigMapTemplate(r, component, k.Spec.Channel).Lookup(ctx)
		if err != nil {
			templates[component.Name] = nil
		}
		templates[component.Name] = configMap
	}
	return templates
}

func (r *KymaReconciler) AreTemplatesOutdated(logger *logr.Logger, k *operatorv1alpha1.Kyma, templates TemplatesByName) bool {
	for componentName, template := range templates {
		for _, condition := range k.Status.Conditions {
			if condition.Reason == componentName && template != nil {
				templateHash := *asHash(template.Data)
				if templateHash != condition.TemplateHash {
					logger.Info("detected outdated template", "condition", condition.Reason, "template", template.Name, "templateHash", templateHash, "oldHash", condition.TemplateHash)
					return true
				}
			}
		}
	}
	return false
}

func asHash(o interface{}) *string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%v", o)))
	v := fmt.Sprintf("%x", h.Sum(nil))
	return &v
}
