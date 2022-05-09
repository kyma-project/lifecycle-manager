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
	"bytes"
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
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
	"text/template"
	"time"
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
	kyma := operatorv1alpha1.Kyma{}

	if err := r.Get(ctx, req.NamespacedName, &kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	kymaObj := &operatorv1alpha1.Kyma{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, kymaObj); err != nil {
		logger.Error(err, "kyma object read error")
	}

	// if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() {
		return ctrl.Result{Requeue: true}, nil
	}

	err := r.onCreateOrUpdate(ctx, req, &kyma)
	if err != nil {
		return ctrl.Result{}, err // retry
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) GetConfigMap(ctx context.Context, component string) (*corev1.ConfigMap, error) {
	configMapList := &corev1.ConfigMapList{}
	if err := r.List(ctx, configMapList, client.MatchingLabels{"operator.kyma-project.io/controller-name": component}); err != nil {
		return nil, err
	}

	if len(configMapList.Items) != 1 {
		return nil, fmt.Errorf("more than one config map found for component: %s", component)
	}

	return &configMapList.Items[0], nil
}

func (r *KymaReconciler) ReconcileFromConfigMap(ctx context.Context, req ctrl.Request, kymaObj operatorv1alpha1.Kyma, progression *KymaProgressionInfo) error {
	var watchInputs []*unstructured.Unstructured
	namespacedName := req.NamespacedName.String()
	logger := log.FromContext(ctx).WithName(namespacedName)

	if len(kymaObj.Spec.Components) < 1 {
		return fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	for _, component := range kymaObj.Spec.Components {
		configMap, err := r.GetConfigMap(ctx, component.Name)
		if err != nil {
			logger.Error(err, "component mapping ConfigMap read error, will not re-queue resource %s", req.NamespacedName.String())
			return err
		}

		componentName := component.Name + "-name"
		//+ uuid.New().String()

		componentBytes, ok := configMap.Data[component.Name]
		if !ok {
			return fmt.Errorf("%s component not found for resource %s", component.Name, namespacedName)
		}

		parsedTemplate, templateErr := template.New(component.Name).Funcs(template.FuncMap{
			"installOperation":     func(interface{}) string { return string(progression.KymaProgressionPath) },
			"installTargetVersion": func(interface{}) string { return progression.New },
			"installOriginVersion": func(interface{}) string { return progression.Old },
		}).Parse(componentBytes)
		if templateErr != nil {
			return fmt.Errorf("error during config map template parsing %w", templateErr)
		}

		templatedData := bytes.NewBufferString("")
		templateExecErr := parsedTemplate.Execute(templatedData, nil)
		if templateExecErr != nil {
			return fmt.Errorf("error during config map template execution %w", templateExecErr)
		}

		componentYaml := make(map[string]interface{})
		if err := yaml.Unmarshal(templatedData.Bytes(), &componentYaml); err != nil {
			return fmt.Errorf("error during config map unmarshal %w", err)
		}

		gvr := schema.GroupVersionResource{
			Group:    componentYaml["group"].(string),
			Resource: componentYaml["resource"].(string),
			Version:  componentYaml["version"].(string),
		}

		res, err := r.GetUnstructuredResource(ctx, gvr, componentName, req.Namespace)

		if client.IgnoreNotFound(err) != nil {
			return err
		}

		componentUnstructured := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       componentYaml["kind"].(string),
				"apiVersion": componentYaml["group"].(string) + "/" + componentYaml["version"].(string),
				"metadata": map[string]interface{}{
					"name":      componentName,
					"namespace": req.Namespace,
					"labels": map[string]interface{}{
						"operator.kyma-project.io/managed-by":      "kyma-operator",
						"operator.kyma-project.io/controller-name": component.Name,
						"operator.kyma-project.io/applied-as":      string(progression.KymaProgressionPath),
						"operator.kyma-project.io/release":         progression.New,
					},
				},
				"spec":   componentYaml["spec"],
				"status": map[string]string{},
			},
		}
		for key, value := range component.Settings {
			componentUnstructured.Object["spec"].(map[string]interface{})[key] = value
		}
		if res != nil {
			if err := r.Client.Patch(ctx, componentUnstructured, client.MergeFromWithOptions(res.DeepCopy(),
				client.MergeFromWithOptimisticLock{})); err != nil {
				return fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully updated component CR of", "type", component.Name)
		} else {

			// set owner reference
			if err := controllerutil.SetOwnerReference(&kymaObj, componentUnstructured, r.Scheme); err != nil {
				return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			if err := r.Client.Create(ctx, componentUnstructured, &client.CreateOptions{}); err != nil {
				return fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			// TODO: implement common watch mechanism for all unstructured kinds
			watchInputs = append(watchInputs, componentUnstructured)

			logger.Info("successfully created component CR of", "type", component.Name)
		}

	}

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
	return nil
}

func (r *KymaReconciler) onCreateOrUpdate(ctx context.Context, req ctrl.Request, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	kyma = kyma.DeepCopy()

	if (kyma.Status.State == operatorv1alpha1.KymaStateReady || kyma.Status.State == operatorv1alpha1.KymaStateError) && kyma.Status.
		ObservedGeneration == kyma.
		ObjectMeta.Generation {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
		return nil
	}

	if kyma.Status.State == operatorv1alpha1.KymaStateProcessing {
		logger.Info("processing " + kyma.Name)
		ready := true
		for _, condition := range kyma.Status.Conditions {
			if condition.Type == operatorv1alpha1.ConditionTypeReady && condition.Status != operatorv1alpha1.ConditionStatusTrue {
				ready = false
				break
			}
		}
		if ready {
			logger.Info(fmt.Sprintf("reconciliation of %s finished!", kyma.Name))
			r.Recorder.Event(kyma, "Normal", "ReconciliationSuccess", fmt.Sprintf("Reconciliation finished!"))
			kyma.Status.ObservedGeneration = kyma.ObjectMeta.Generation
			kyma.Status.State = operatorv1alpha1.KymaStateReady
			return r.updateKymaStatus(ctx, kyma)
		}
	}

	kyma.Status.State = operatorv1alpha1.KymaStateProcessing
	kyma.Status.ObservedGeneration = kyma.ObjectMeta.Generation

	//THIS IS JUST STUB CODE
	if len(kyma.Status.Conditions) == 0 {
		logger.Info(fmt.Sprintf("fresh kyma installation %s, adding reconciliation conditions!", kyma.Name))
		kyma.Status.Conditions = append(kyma.Status.Conditions, operatorv1alpha1.KymaCondition{
			Type:               operatorv1alpha1.ConditionTypeReady,
			Status:             operatorv1alpha1.ConditionStatusFalse,
			Message:            "reconciliation has to be finished",
			Reason:             "reconciliation triggered",
			LastTransitionTime: &metav1.Time{Time: time.Now()},
		})
		r.Recorder.Event(kyma, "Normal", "Updated", fmt.Sprintf("Reconciliation started"))
		return r.updateKymaStatus(ctx, kyma)
	} else {
		return r.reconcileKymaForRelease(ctx, req, kyma)
	}
}

func (r *KymaReconciler) reconcileKymaForRelease(ctx context.Context, req ctrl.Request, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)
	kyma.Status.Conditions[0].Status = operatorv1alpha1.ConditionStatusTrue
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
	if err := r.ReconcileFromConfigMap(ctx, req, *kyma, &KymaProgressionInfo{
		KymaProgressionPath: path,
		Old:                 oldRelease,
		New:                 newRelease,
	}); err != nil {
		failureReason := fmt.Sprintf("CR creation error: %s", err.Error())
		logger.Info(failureReason)
		kyma.Status.ObservedGeneration = kyma.ObjectMeta.Generation
		kyma.Status.State = operatorv1alpha1.KymaStateError
		r.Recorder.Event(kyma, "Warning", "ReconciliationFailed", fmt.Sprintf("Reconciliation failed: %s",
			failureReason))
		statusUpdateErr := r.updateKymaStatus(ctx, kyma)
		logger.Error(statusUpdateErr, "error ocurred after updating status for a failed reconciliation")
		return err
	}

	kyma.Status.ActiveRelease = newRelease

	return r.updateKymaStatus(ctx, kyma)
}

func (r *KymaReconciler) updateKyma(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Update(ctx, kyma)
	})
}

func (r *KymaReconciler) updateKymaStatus(ctx context.Context, kyma *operatorv1alpha1.Kyma) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Status().Update(ctx, kyma)
	})
}

func (r *KymaReconciler) GetUnstructuredResource(ctx context.Context, gvr schema.GroupVersionResource, name string, namespace string) (*unstructured.Unstructured, error) {
	config, err := GetConfig()
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name,
		metav1.GetOptions{})
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
		if key == "state" && value == operatorv1alpha1.KymaStateReady {
			kymaObj := operatorv1alpha1.Kyma{}
			ownerRefs := componentObj.GetOwnerReferences()
			var ownerName string

			for _, ownerRef := range ownerRefs {
				if kymaObj.Kind == ownerRef.Kind {
					ownerName = ownerRef.Name
					break
				}
			}

			if err := r.Get(context.TODO(), types.NamespacedName{Name: ownerName, Namespace: componentObj.GetNamespace()}, &kymaObj); err != nil {
				return
			}

			kymaObj.Status.State = operatorv1alpha1.KymaStateReady
			kymaObj.Status.ObservedGeneration = kymaObj.Generation
			var condition *operatorv1alpha1.KymaCondition

			for _, existingCondition := range kymaObj.Status.Conditions {
				if existingCondition.Type == operatorv1alpha1.ConditionTypeReady {
					condition = &existingCondition
				}
			}

			if condition == nil {
				condition = &operatorv1alpha1.KymaCondition{
					Type: operatorv1alpha1.ConditionTypeReady,
				}
				kymaObj.Status.Conditions = append(kymaObj.Status.Conditions, *condition)
			}

			condition.LastTransitionTime = &metav1.Time{Time: time.Now()}
			condition.Message = "successfully installed component type: " + e.ObjectNew.GetObjectKind().GroupVersionKind().String()
			condition.Reason = "all component installed"
			condition.Status = operatorv1alpha1.ConditionStatusTrue

			if err := r.Status().Update(context.TODO(), &kymaObj); err != nil {
				return
			}
		}
	}
}
