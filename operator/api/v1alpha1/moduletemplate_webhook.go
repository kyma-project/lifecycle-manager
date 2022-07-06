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

package v1alpha1

import (
	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var moduletemplatelog = logf.Log.WithName("moduletemplate-resource")

func (r *ModuleTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1alpha1-moduletemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1alpha1,name=mmoduletemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ModuleTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *ModuleTemplate) Default() {
	moduletemplatelog.Info("default", "name", r.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1alpha1-moduletemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1alpha1,name=vmoduletemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ModuleTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ModuleTemplate) ValidateCreate() error {
	moduletemplatelog.Info("validate create", "name", r.Name)
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ModuleTemplate) ValidateUpdate(old runtime.Object) error {
	moduletemplatelog.Info("validate update", "name", r.Name)
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ModuleTemplate) ValidateDelete() error {
	moduletemplatelog.Info("validate delete", "name", r.Name)
	return r.validate()
}

func (r *ModuleTemplate) validate() error {
	var allErrs field.ErrorList
	if err := r.validateDescriptor(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: GroupVersion.Group, Kind: "ModuleTemplate"},
		r.Name, allErrs)
}

func (r *ModuleTemplate) validateDescriptor() *field.Error {
	var descriptor ocm.ComponentDescriptor
	if err := codec.Decode(r.Spec.Descriptor.Raw, &descriptor); err != nil {
		return field.Invalid(field.NewPath("spec").Child("descriptor"), string(r.Spec.Descriptor.Raw), err.Error())
	}
	return nil
}
