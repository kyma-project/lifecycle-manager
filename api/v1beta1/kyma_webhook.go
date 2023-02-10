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
var kymalog = logf.Log.WithName("kyma-resource") //nolint:gochecknoglobals

func (kyma *Kyma) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(kyma).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
//nolint:lll
//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1beta1-kyma,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=kymas,verbs=create;update,versions=v1beta1,name=mkyma.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Kyma{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (kyma *Kyma) Default() {
	kymalog.Info("default", "name", kyma.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//nolint:lll
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1beta1-kyma,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=kymas,verbs=create;update,versions=v1beta1,name=vkyma.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Kyma{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (kyma *Kyma) ValidateCreate() error {
	kymalog.Info("validate create", "name", kyma.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (kyma *Kyma) ValidateUpdate(_ runtime.Object) error {
	kymalog.Info("validate update", "name", kyma.Name)
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (kyma *Kyma) ValidateDelete() error {
	kymalog.Info("validate delete", "name", kyma.Name)
	return nil
}
