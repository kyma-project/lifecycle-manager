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
	"context"
	ocm "github.com/gardener/component-spec/bindings-go/apis/v2"
	"github.com/gardener/component-spec/bindings-go/codec"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var moduletemplatelog = logf.Log.WithName("moduletemplate-resource") //nolint:gochecknoglobals

func (moduleTemplate *ModuleTemplate) SetupWebhookWithManager(
	mgr ctrl.Manager,
) error {
	return ctrl.NewWebhookManagedBy(mgr).WithValidator(&clusterAwareModuleTemplateValidator{
		Client: mgr.GetClient(),
	}).For(moduleTemplate).Complete()
}

type clusterAwareModuleTemplateValidator struct {
	Client client.Client
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//nolint:lll
//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1alpha1-moduletemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1alpha1,name=mmoduletemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ModuleTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (moduleTemplate *ModuleTemplate) Default() {
	moduletemplatelog.Info("default", "name", moduleTemplate.Name)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//nolint:lll
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1alpha1-moduletemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1alpha1,name=vmoduletemplate.kb.io,admissionReviewVersions=v1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *clusterAwareModuleTemplateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	moduletemplatelog.Info("validate create", "name", obj.(*ModuleTemplate).Name)
	return r.validate(ctx, obj.(*ModuleTemplate))
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *clusterAwareModuleTemplateValidator) ValidateUpdate(
	ctx context.Context, oldObj runtime.Object, newObj runtime.Object,
) error {
	moduletemplatelog.Info("validate update", "name", newObj.(*ModuleTemplate).Name)
	return r.validate(ctx, newObj.(*ModuleTemplate))
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *clusterAwareModuleTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	moduletemplatelog.Info("validate delete", "name", obj.(*ModuleTemplate).Name)
	return r.validate(ctx, obj.(*ModuleTemplate))
}

func (r *clusterAwareModuleTemplateValidator) validate(_ context.Context, template *ModuleTemplate) error {
	var allErrs field.ErrorList
	if err := r.validateDescriptor(template); err != nil {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: GroupVersion.Group, Kind: "ModuleTemplate"},
		template.Name, allErrs)
}

func (r *clusterAwareModuleTemplateValidator) validateDescriptor(template *ModuleTemplate) *field.Error {
	var descriptor ocm.ComponentDescriptor
	if err := codec.Decode(template.Spec.OCMDescriptor.Raw, &descriptor); err != nil {
		return field.Invalid(field.NewPath("spec").Child("descriptor"), string(template.Spec.OCMDescriptor.Raw), err.Error())
	}
	return nil
}
