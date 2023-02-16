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

//nolint:dupl
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var watcherlog = logf.Log.WithName("watcher-resource") //nolint:gochecknoglobals

func (w *Watcher) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(w).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
//nolint:lll
//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1beta1-watcher,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=watchers,verbs=create;update,versions=v1beta1,name=mwatcher.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Watcher{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (w *Watcher) Default() {
	watcherlog.Info("default", "name", w.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//nolint:lll
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1beta1-watcher,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=watchers,verbs=create;update,versions=v1beta1,name=vwatcher.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Watcher{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (w *Watcher) ValidateCreate() error {
	watcherlog.Info("validate create", "name", w.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (w *Watcher) ValidateUpdate(_ runtime.Object) error {
	watcherlog.Info("validate update", "name", w.Name)

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (w *Watcher) ValidateDelete() error {
	watcherlog.Info("validate delete", "name", w.Name)

	return nil
}
