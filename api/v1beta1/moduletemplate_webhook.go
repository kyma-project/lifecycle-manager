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

package v1beta1

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

//nolint:lll
//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1beta1-moduletemplate,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1beta1,name=v1beta1.mmoduletemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &ModuleTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (in *ModuleTemplate) Default() {
	moduletemplatelog.Info("default", "name", in.Name)
}

//nolint:lll
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1beta1-moduletemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1beta1,name=v1beta1.vmoduletemplate.kb.io,admissionReviewVersions=v1

// log is for logging in this package.
var moduletemplatelog = logf.Log.WithName("moduletemplate-resource") //nolint:gochecknoglobals

func (in *ModuleTemplate) SetupWebhookWithManager(
	mgr ctrl.Manager,
) error {
	return ctrl.NewWebhookManagedBy(mgr).WithValidator(
		&clusterAwareModuleTemplateValidator{
			Client: mgr.GetClient(),
		},
	).For(in).Complete()
}

type clusterAwareModuleTemplateValidator struct {
	Client client.Client
}

func (c *clusterAwareModuleTemplateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	moduletemplatelog.Info("validate create", "name", obj.(*ModuleTemplate).Name)
	return c.validate(ctx, nil, obj.(*ModuleTemplate))
}

func (c *clusterAwareModuleTemplateValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	moduletemplatelog.Info("validate update", "name", newObj.(*ModuleTemplate).Name)
	return c.validate(ctx, oldObj.(*ModuleTemplate), newObj.(*ModuleTemplate))
}

func (c *clusterAwareModuleTemplateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	moduletemplatelog.Info("validate delete", "name", obj.(*ModuleTemplate).Name)
	return c.validate(ctx, nil, obj.(*ModuleTemplate))
}

func (c *clusterAwareModuleTemplateValidator) validate(
	_ context.Context, oldTemplate, newTemplate *ModuleTemplate,
) error {
	newDescriptor, err := newTemplate.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	if err := compdesc.Validate(newDescriptor.ComponentDescriptor); err != nil {
		return err
	}

	newVersion, err := semver.NewVersion(newDescriptor.Version)
	if err != nil {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "ModuleTemplate"},
			newTemplate.Name, field.ErrorList{field.Invalid(
				field.NewPath("spec").Child("descriptor").
					Child("version"),
				newDescriptor.Version, err.Error(),
			)},
		)
	}

	if oldTemplate != nil {
		// the old descriptor has to be valid since it otherwise would not have been submitted
		oldDescriptor, _ := oldTemplate.Spec.GetDescriptor()
		oldVersion, _ := semver.NewVersion(oldDescriptor.Version)
		if newVersion.LessThan(oldVersion) {
			return apierrors.NewInvalid(
				schema.GroupKind{Group: GroupVersion.Group, Kind: "ModuleTemplate"},
				newTemplate.Name, field.ErrorList{field.Invalid(
					field.NewPath("spec").Child("descriptor").
						Child("version"),
					newVersion.String(), fmt.Sprintf(
						"version of templates can never be decremented (previously %s)",
						oldVersion,
					),
				)},
			)
		}
	}

	return nil
}
