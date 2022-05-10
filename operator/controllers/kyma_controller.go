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
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Builder  *builder.Builder
	Recorder record.EventRecorder
}

type KymaProgressionPath string

type KymaProgressionInfo struct {
	KymaProgressionPath `json:"progressionPath"`
	Old                 string `json:"old"`
	New                 string `json:"new"`
}

const (
	KymaUpgrade   KymaProgressionPath = "upgrade"
	KymaDowngrade KymaProgressionPath = "downgrade"
	KymaInstall   KymaProgressionPath = "install"
	KymaUpdate    KymaProgressionPath = "update"
	KymaKind      string              = "Kyma"
)

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

func (r *KymaReconciler) HandleInitialState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	return nil
}

func (r *KymaReconciler) HandleProcessingState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	logger.Info("processing " + kyma.Name)

	if areAllReadyConditionsSet(kyma) {
		message := fmt.Sprintf("reconciliation of %s finished!", kyma.Name)
		logger.Info(message)
		r.Recorder.Event(kyma, "Normal", "ReconciliationSuccess", message)
		return r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateReady, message)
	}

	return r.reconcileKymaForRelease(ctx, kyma)
}

func (r *KymaReconciler) HandleDeletingState(ctx context.Context) error {
	return nil
}

func (r *KymaReconciler) HandleErrorState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	if kyma.Status.ObservedGeneration == kyma.Generation {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
	}
	return nil
}

func (r *KymaReconciler) HandleReadyState(ctx context.Context, logger *logr.Logger, kyma *operatorv1alpha1.Kyma) error {
	if kyma.Status.ObservedGeneration == kyma.Generation {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
	}
	return nil
}

func (r *KymaReconciler) updateKyma(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return r.Update(ctx, kyma)
}

func (r *KymaReconciler) updateKymaStatus(ctx context.Context, kyma *operatorv1alpha1.Kyma, state operatorv1alpha1.KymaState, message string) error {
	switch state {
	case operatorv1alpha1.KymaStateReady:
		addReadyConditionForObjects(kyma, []string{KymaKind}, operatorv1alpha1.ConditionStatusTrue, message)
		// set active release only when ready state is set
		SetActiveRelease(kyma)
	case "":
		addReadyConditionForObjects(kyma, []string{KymaKind}, operatorv1alpha1.ConditionStatusUnknown, message)
	default:
		addReadyConditionForObjects(kyma, []string{KymaKind}, operatorv1alpha1.ConditionStatusFalse, message)
	}
	return r.Status().Update(ctx, SetObservedGeneration(kyma))
}

func (r *KymaReconciler) GetTemplateConfigMapForRelease(ctx context.Context, component, release string) (*corev1.ConfigMap, error) {
	configMapList := &corev1.ConfigMapList{}
	if err := r.List(ctx, configMapList,
		client.MatchingLabels{
			"operator.kyma-project.io/controller-name": component,
			"operator.kyma-project.io/release":         release,
		},
	); err != nil {
		return nil, err
	}

	if len(configMapList.Items) > 1 {
		return nil, fmt.Errorf("more than one config map template found for component: %s", component)
	}

	if len(configMapList.Items) == 0 {
		log.FromContext(ctx).Info(fmt.Sprintf("no template for component %s found for release %s, falling back to any found config map template", component, release))

		if err := r.List(ctx, configMapList,
			client.MatchingLabels{
				"operator.kyma-project.io/controller-name": component,
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

	return &configMapList.Items[0], nil
}

func (r *KymaReconciler) ReconcileFromConfigMap(ctx context.Context, kymaObj *operatorv1alpha1.Kyma, release *KymaProgressionInfo) error {
	var watchInputs []*unstructured.Unstructured
	kymaObjectKey := client.ObjectKey{Name: kymaObj.Name, Namespace: kymaObj.Namespace}
	namespacedName := kymaObjectKey.String()
	logger := log.FromContext(ctx).WithName(namespacedName)

	if len(kymaObj.Spec.Components) < 1 {
		return fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	var componentNames []string
	for _, component := range kymaObj.Spec.Components {
		configMap, err := r.GetTemplateConfigMapForRelease(ctx, component.Name, release.New)
		componentNames = append(componentNames, component.Name)
		if err != nil {
			logger.Error(err, fmt.Sprintf("could not find template configmap for resource %s and release %s, will not re-queue resource %s", component.Name, release.New, namespacedName))
			return err
		}

		componentName := component.Name + "-name"

		componentBytes, ok := configMap.Data[component.Name]
		if !ok {
			return fmt.Errorf("%s component not found for resource %s", component.Name, namespacedName)
		}

		componentYaml, templateErr := r.GetTemplatedComponent(componentBytes)
		if templateErr != nil {
			return fmt.Errorf("error during config map template parsing %w", templateErr)
		}

		res := unstructured.Unstructured{}
		res.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   componentYaml["group"].(string),
			Kind:    componentYaml["kind"].(string),
			Version: componentYaml["version"].(string),
		})

		err = r.Get(ctx, client.ObjectKey{Namespace: kymaObj.Namespace, Name: componentName}, &res)
		if client.IgnoreNotFound(err) != nil {
			return err
		}

		// overwrite labels for upgrade / downgrade of component versions
		// KymaUpdate doesn't require an update
		if !errors.IsNotFound(err) {
			if release.KymaProgressionPath != KymaUpdate {
				// set labels
				SetComponentCRLabels(&res, component.Name, *release)

				if err := r.Client.Update(ctx, &res); err != nil {
					return fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
				}

				logger.Info("successfully updated component CR of", "type", component.Name)
			}
		} else {
			componentUnstructured := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind":       componentYaml["kind"].(string),
					"apiVersion": componentYaml["group"].(string) + "/" + componentYaml["version"].(string),
					"metadata": map[string]interface{}{
						"name":      componentName,
						"namespace": kymaObj.Namespace,
						"labels":    map[string]interface{}{},
					},
					"spec": componentYaml["spec"],
				},
			}
			for key, value := range component.Settings {
				componentUnstructured.Object["spec"].(map[string]interface{})[key] = value
			}

			// set labels
			SetComponentCRLabels(componentUnstructured, component.Name, *release)

			// set owner reference
			if err := controllerutil.SetOwnerReference(kymaObj, componentUnstructured, r.Scheme); err != nil {
				return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			// create resource if not found
			if err := r.Client.Create(ctx, componentUnstructured, &client.CreateOptions{}); err != nil {
				return fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			// TODO: implement common watch mechanism for all unstructured kinds
			watchInputs = append(watchInputs, componentUnstructured)

			logger.Info("successfully created component CR of", "type", component.Name)
		}
	}

	// TODO: watch all allocated types during controller startup
	for _, addedComponent := range watchInputs {
		r.Builder.
			Watches(
				&source.Kind{Type: addedComponent},
				handler.Funcs{
					UpdateFunc: r.ComponentChangeHandler,
				}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
			)
		logger.Info("successfully created component CR of", "type", addedComponent.GetKind())
	}

	if len(watchInputs) > 0 {
		if err := r.Builder.Complete(r); err != nil {
			logger.Error(err, "error while assigning watch update event on component CRs")
		}
	}

	// check component conditions, if not present add them
	logger.Info("checking condition for component CRs")
	addReadyConditionForObjects(kymaObj, componentNames, operatorv1alpha1.ConditionStatusFalse, "initial condition for component CR")
	return r.updateKymaStatus(ctx, kymaObj, kymaObj.Status.State, "")
}

func (r *KymaReconciler) GetTemplatedComponent(componentTemplate string) (map[string]interface{}, error) {
	componentYaml := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(componentTemplate), &componentYaml); err != nil {
		return nil, fmt.Errorf("error during config map unmarshal %w", err)
	}
	return componentYaml, nil
}

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	oldRelease, newRelease := kyma.Status.ActiveRelease, kyma.Spec.Release

	compared := semver.Compare(oldRelease, newRelease)
	var path KymaProgressionPath
	if compared < 0 {
		if oldRelease == "" {
			path = KymaInstall
			r.Recorder.Event(kyma, "Normal", "ReconciliationUpgrade", fmt.Sprintf("Initial Installation: %s", newRelease))
		} else {
			path = KymaUpgrade
			r.Recorder.Event(kyma, "Normal", "ReconciliationUpgrade", fmt.Sprintf("Upgrade from %s to %s",
				oldRelease, newRelease))
		}
	} else if compared > 0 {
		path = KymaDowngrade
		r.Recorder.Event(kyma, "Normal", "ReconciliationDowngrade", fmt.Sprintf("Downgrade from %s to %s",
			oldRelease, newRelease))
	} else {
		path = KymaUpdate
		r.Recorder.Event(kyma, "Normal", "ReconciliationUpdate", fmt.Sprintf("Update Active Release %s",
			kyma.Spec.Release))
	}

	// read config map
	if err := r.ReconcileFromConfigMap(ctx, kyma, &KymaProgressionInfo{
		KymaProgressionPath: path,
		Old:                 oldRelease,
		New:                 newRelease,
	}); err != nil {
		message := fmt.Sprintf("CR creation error: %s", err.Error())
		logger.Info(message)
		r.Recorder.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s", message))

		if err := r.updateKymaStatus(ctx, kyma, operatorv1alpha1.KymaStateError, message); err != nil {
			return err
		}

		return err
	}

	//kyma.Status.ActiveRelease = newRelease
	//
	//return r.updateKymaStatus(ctx, kyma)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Builder = ctrl.NewControllerManagedBy(mgr)
	return r.Builder.For(&operatorv1alpha1.Kyma{}).Complete(r)
}

func (r *KymaReconciler) ComponentChangeHandler(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	objectBytes, err := json.Marshal(e.ObjectNew)
	if err != nil {
		panic(err)
	}
	componentObj := unstructured.Unstructured{}
	if err = json.Unmarshal(objectBytes, &componentObj); err != nil {
		panic(err)
	}
	if componentObj.Object["status"] == nil {
		return
	}
	for key, value := range componentObj.Object["status"].(map[string]interface{}) {
		if key == "state" && value == "Ready" {
			ownerRefs := componentObj.GetOwnerReferences()
			var ownerName string
			kymaObj := &operatorv1alpha1.Kyma{}
			for _, ownerRef := range ownerRefs {
				if KymaKind == ownerRef.Kind {
					ownerName = ownerRef.Name
					break
				}
			}

			// TODO: propagate context from Reconcile() function
			if err := r.Get(context.TODO(), types.NamespacedName{Name: ownerName, Namespace: componentObj.GetNamespace()}, kymaObj); err != nil {
				return
			}

			kymaObj.Status.State = operatorv1alpha1.KymaStateReady
			kymaObj.Status.ObservedGeneration = kymaObj.Generation

			componentNameLabel := componentObj.GetLabels()["operator.kyma-project.io/controller-name"]
			if componentNameLabel == "" {
				return
			}

			// TODO: "istio", "serverless" are hard-coded, remove!
			addReadyConditionForObjects(kymaObj, []string{componentNameLabel, "istio", "serverless"}, operatorv1alpha1.ConditionStatusTrue,
				fmt.Sprintf("successfully installed component : %s", e.ObjectNew.GetObjectKind().GroupVersionKind().String()))

			// TODO: propagate context from Reconcile() function
			if err := r.updateKymaStatus(context.TODO(), kymaObj, operatorv1alpha1.KymaStateReady, "all components are in ready state"); err != nil {
				return
			}
		}
	}
}
