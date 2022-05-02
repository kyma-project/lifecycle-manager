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
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
)

// KymaReconciler reconciles a Kyma object
type KymaReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Builder *builder.Builder
}

//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.kyma-project.io,resources=kymas/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *KymaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName(req.NamespacedName.String())
	logger.Info("Reconciliation loop starting for", "resource", req.NamespacedName.String())

	kymaObj := &operatorv1alpha1.Kyma{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, kymaObj); err != nil {
		logger.Error(err, "kyma object read error")
	}

	configMap, err := r.GetConfigMap(req)
	if apiErrors.IsNotFound(err) {
		logger.Error(err, "component mapping ConfigMap not found, re-queuing resource %s", req.NamespacedName.String())
		return ctrl.Result{}, err
	} else if err != nil {
		logger.Error(err, "component mapping ConfigMap read error, will not re-queue resource %s", req.NamespacedName.String())
		return ctrl.Result{}, err
	}

	// read config map
	if err := r.ReconcileFromConfigMap(ctx, req, configMap, *kymaObj); err != nil {
		logger.Error(err, "component CR creation error")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *KymaReconciler) GetConfigMap(req ctrl.Request) (*corev1.ConfigMap, error) {
	configMapNamespaced := types.NamespacedName{Name: "kyma-component-config", Namespace: req.Namespace}

	configMap := &corev1.ConfigMap{}

	return configMap, r.Get(context.TODO(), configMapNamespaced, configMap)
}

func (r *KymaReconciler) ReconcileFromConfigMap(ctx context.Context, req ctrl.Request, configMap *corev1.ConfigMap, kymaObj operatorv1alpha1.Kyma) error {
	namespacedName := req.NamespacedName.String()
	logger := log.FromContext(ctx).WithName(namespacedName)

	if len(kymaObj.Spec.Components) < 1 {
		return fmt.Errorf("no component specified for resource %s", namespacedName)
	}

	for _, component := range kymaObj.Spec.Components {
		componentName := component.Name + "-name"

		componentBytes, ok := configMap.Data[component.Name]
		if !ok {
			return fmt.Errorf("%s component not found for resource %s", component.Name, namespacedName)
		}

		componentYaml := make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(componentBytes), &componentYaml); err != nil {
			return fmt.Errorf("error during config map unmarshal %w", err)
		}

		gvr := schema.GroupVersionResource{
			Group:    componentYaml["group"].(string),
			Resource: componentYaml["resource"].(string),
			Version:  componentYaml["version"].(string),
		}

		if exists, err := r.GetUnstructuredResource(ctx, gvr, componentName, req.Namespace); client.IgnoreNotFound(err) != nil {
			return err
		} else if exists {
			continue
		}

		componentUnstructured := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"kind":       componentYaml["kind"].(string),
				"apiVersion": componentYaml["group"].(string) + "/" + componentYaml["version"].(string),
				"metadata": map[string]interface{}{
					"name":      componentName,
					"namespace": req.Namespace,
				},
				"spec": componentYaml["spec"],
			},
		}

		// set owner reference
		if err := controllerutil.SetOwnerReference(&kymaObj, componentUnstructured, r.Scheme); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w", component.Name, namespacedName, err)
		}

		if err := r.Client.Create(context.Background(), componentUnstructured, &client.CreateOptions{}); err != nil {
			return fmt.Errorf("error creating custom resource of type %s %w", component.Name, err)
		}

		logger.Info("successfully created component CR of", "type", component.Name)
	}

	return nil
}

func (r *KymaReconciler) GetUnstructuredResource(ctx context.Context, gvr schema.GroupVersionResource, name string, namespace string) (bool, error) {
	config, err := GetConfig()
	if err != nil {
		return false, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return false, err
	}

	if _, err = dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{}); err != nil {
		return false, err
	}

	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KymaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Builder = ctrl.NewControllerManagedBy(mgr)
	return r.Builder.For(&operatorv1alpha1.Kyma{}).
		//Owns(&corev1.ConfigMap{}).
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForConfigMap),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *KymaReconciler) findObjectsForConfigMap(configMap client.Object) []reconcile.Request {
	requests := make([]reconcile.Request, 0)
	// add code here to watch for changes in component mapping ConfigMap
	return requests
}
