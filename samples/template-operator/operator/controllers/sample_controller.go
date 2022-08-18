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
	"github.com/kyma-project/kyma-operator/samples/template-operator/api/v1alpha1"
	"github.com/kyma-project/manifest-operator/operator/pkg/declarative"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SampleReconciler reconciles a Sample object
type SampleReconciler struct {
	declarative.ManifestReconciler
	client.Client
	Scheme *runtime.Scheme
	*rest.Config
}

const deletionFinalizer = "deletion-finalizer"

//+kubebuilder:rbac:groups=component.kyma-project.io,resources=samples,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=samples/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=component.kyma-project.io,resources=samples/finalizers,verbs=update

// SetupWithManager sets up the controller with the Manager.
func (r *SampleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Config = mgr.GetConfig()
	if err := r.Inject(mgr, &v1alpha1.Sample{},
		declarative.WithResourceLabels(
			map[string]string{"sampleKey": "sampleValue"},
		)); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Sample{}).
		Complete(r)
}
