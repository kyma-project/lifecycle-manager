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
	"github.com/kyma-project/kyma-operator/operator/pkg/dynamic"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"text/template"
	"time"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Builder  *builder.Builder
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
	kyma := operatorv1alpha1.Kyma{}

	if err := r.Get(ctx, req.NamespacedName, &kyma); err != nil {
		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		logger.Info(req.NamespacedName.String() + " got deleted!")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// if deletionTimestamp is set, retry until it gets fully deleted
	if !kyma.DeletionTimestamp.IsZero() {
		return ctrl.Result{Requeue: true}, nil
	}

	if kyma.IsAlreadyReconciled() {
		logger.Info("skipping reconciliation for " + kyma.Name + ", already reconciled!")
		return ctrl.Result{}, nil
	}

	err := r.onCreateOrUpdate(ctx, req, &kyma)
	if err != nil {
		return ctrl.Result{}, err // retry
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) ReconcileFromConfigMap(ctx context.Context, req ctrl.Request, kyma *operatorv1alpha1.Kyma,
	rel *release.Kyma) error {
	namespacedName := req.NamespacedName.String()
	logger := log.FromContext(ctx).WithName(namespacedName)

	if len(kyma.Spec.Components) < 1 {
		return fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	finder := dynamic.NewTemplateFinder(r).Release(rel.New)

	for _, component := range kyma.Spec.Components {
		configMap, err := finder.For(component.Name).Find(ctx)
		if err != nil {
			logger.Error(err, fmt.Sprintf("could not find template configmap for resource %s and release %s, will not re-queue resource %s", component.Name, rel.New, req.NamespacedName.String()))
			return err
		}

		componentName := component.Name + "-name"

		componentBytes, ok := configMap.Data[component.Name]
		if !ok {
			return fmt.Errorf("%s component not found for resource %s", component.Name, namespacedName)
		}

		componentYaml, templateErr := r.GetTemplatedComponent(component.Name, componentBytes, rel)
		if templateErr != nil {
			return fmt.Errorf("error during config map template parsing %w", templateErr)
		}

		res, err := dynamic.GetUnstructuredResource(ctx, schema.GroupVersionResource{
			Group:    componentYaml["group"].(string),
			Resource: componentYaml["resource"].(string),
			Version:  componentYaml["version"].(string),
		}, componentName, req.Namespace)

		if client.IgnoreNotFound(err) != nil {
			return err
		}

		//goland:noinspection GoRedundantConversion
		componentUnstructured := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       componentYaml["kind"].(string),
				"apiVersion": componentYaml["group"].(string) + "/" + componentYaml["version"].(string),
				"metadata": map[string]interface{}{
					"name":      componentName,
					"namespace": req.Namespace,
					"labels": map[string]interface{}{
						labels.ManagedBy:      "kyma-operator",
						labels.ControllerName: component.Name,
						labels.AppliedAs:      string(rel.Type),
						labels.Release:        rel.New,
					},
				},
				"spec": componentYaml["spec"],
			},
		}

		if res != nil {
			if err := r.Client.Patch(ctx, componentUnstructured, client.MergeFromWithOptions(res.DeepCopy(),
				client.MergeFromWithOptimisticLock{})); err != nil {
				return fmt.Errorf("error updating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully updated component CR of", "type", component.Name)
		} else {
			// set owner reference
			if err := controllerutil.SetOwnerReference(kyma, componentUnstructured, r.Scheme); err != nil {
				return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
			}

			if err := r.Client.Create(ctx, componentUnstructured, &client.CreateOptions{}); err != nil {
				return fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
			}

			logger.Info("successfully created component CR of", "type", component.Name)
		}
	}

	return nil
}

func (r *KymaReconciler) GetTemplatedComponent(templateName, componentTemplate string, rel *release.Kyma) (map[string]interface{}, error) {
	parsedTemplate, templateErr := template.New(templateName).Funcs(template.FuncMap{
		"installOperation":     func(interface{}) release.Type { return rel.Type },
		"installTargetVersion": func(interface{}) string { return rel.New },
		"installOriginVersion": func(interface{}) string { return rel.Old },
	}).Parse(componentTemplate)
	if templateErr != nil {
		return nil, fmt.Errorf("error during config map template parsing %w", templateErr)
	}

	templatedData := bytes.NewBufferString("")
	templateExecErr := parsedTemplate.Execute(templatedData, nil)
	if templateExecErr != nil {
		return nil, fmt.Errorf("error during config map template execution %w", templateExecErr)
	}

	componentYaml := make(map[string]interface{})
	if err := yaml.Unmarshal(templatedData.Bytes(), &componentYaml); err != nil {
		return nil, fmt.Errorf("error during config map unmarshal %w", err)
	}

	return componentYaml, nil
}

func (r *KymaReconciler) onCreateOrUpdate(ctx context.Context, req ctrl.Request, kyma *operatorv1alpha1.Kyma) error {
	logger := log.FromContext(ctx)

	if kyma.IsReconciling() {
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
			Reason:             "ReconciliationTriggered",
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

	rel := release.NewKyma(kyma.Status.ActiveRelease, kyma.Spec.Release, func(eventtype, reason, message string) {
		r.Recorder.Event(kyma, eventtype, reason, message)
	})

	// read config map
	if err := r.ReconcileFromConfigMap(ctx, req, kyma, rel); err != nil {
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

	kyma.Status.ActiveRelease = rel.New

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

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Builder = ctrl.NewControllerManagedBy(mgr)
	return r.Builder.For(&operatorv1alpha1.Kyma{}).
		//Owns(&corev1.ConfigMap{}).
		//Watches(
		//	&source.Kind{CalculateReleaseType: &corev1.ConfigMap{}},
		//	handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
		//	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		//).
		Complete(r)
}

func (r *KymaReconciler) findObjectsForConfigMap(configMap client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0)
	// add code here to watch for changes in component mapping ConfigMap
	return requests
}
