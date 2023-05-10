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

package v1beta2

import (
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var (
	kymalog            = logf.Log.WithName("kyma-resource")
	ErrDuplicateModule = errors.New("duplicate module")
)

func (kyma *Kyma) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(kyma).
		Complete()
}

//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1beta2-kyma,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=kymas,verbs=create;update,versions=v1beta2,name=vkyma.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Kyma{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (kyma *Kyma) ValidateCreate() error {
	kymalog.Info("validate create", "name", kyma.Name)
	return ValidateKymaModule(kyma)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (kyma *Kyma) ValidateUpdate(_ runtime.Object) error {
	kymalog.Info("validate update", "name", kyma.Name)
	return ValidateKymaModule(kyma)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (kyma *Kyma) ValidateDelete() error {
	kymalog.Info("validate delete", "name", kyma.Name)
	return nil
}

func ValidateKymaModule(kyma *Kyma) error {
	moduleSet := map[string]bool{}
	for _, module := range kyma.Spec.Modules {
		if _, found := moduleSet[module.Name]; found {
			return apierrors.NewInvalid(
				schema.GroupKind{Group: GroupVersion.Group, Kind: string(KymaKind)},
				kyma.Name, field.ErrorList{field.Invalid(
					field.NewPath("spec").Child("modules"),
					module.Name, ErrDuplicateModule.Error())})
		} else {
			moduleSet[module.Name] = true
		}
	}
	return nil
}
