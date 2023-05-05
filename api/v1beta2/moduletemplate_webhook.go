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
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/open-component-model/ocm/pkg/contexts/ocm/compdesc"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var moduletemplatelog = logf.Log.WithName("moduletemplate-resource") //nolint:gochecknoglobals

func (m *ModuleTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

//nolint:lll
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1beta2-moduletemplate,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=moduletemplates,verbs=create;update,versions=v1beta2,name=v1beta2.vmoduletemplate.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ModuleTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *ModuleTemplate) ValidateCreate() error {
	moduletemplatelog.Info("validate create", "name", m.Name)
	newDescriptor, err := m.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	return Validate(nil, newDescriptor, m.Name)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *ModuleTemplate) ValidateUpdate(old runtime.Object) error {
	moduletemplatelog.Info("validate update", "name", m.Name)
	newDescriptor, err := m.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	oldTemplate := old.(*ModuleTemplate)
	oldDescriptor, err := oldTemplate.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	return Validate(oldDescriptor, newDescriptor, m.Name)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *ModuleTemplate) ValidateDelete() error {
	moduletemplatelog.Info("validate delete", "name", m.Name)
	newDescriptor, err := m.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	return Validate(nil, newDescriptor, m.Name)
}

func Validate(oldDescriptor, newDescriptor *Descriptor, newTemplateName string) error {
	if err := compdesc.Validate(newDescriptor.ComponentDescriptor); err != nil {
		return err
	}

	newVersion, err := semver.NewVersion(newDescriptor.Version)
	if err != nil {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "ModuleTemplate"},
			newTemplateName, field.ErrorList{field.Invalid(
				field.NewPath("spec").Child("descriptor").
					Child("version"),
				newDescriptor.Version, err.Error(),
			)},
		)
	}

	if oldDescriptor != nil {
		// the old descriptor has to be valid since it otherwise would not have been submitted
		oldVersion, err := semver.NewVersion(oldDescriptor.Version)
		if err != nil {
			return err
		}
		return validateVersionUpgrade(newVersion, oldVersion, newTemplateName)
	}

	return nil
}

func validateVersionUpgrade(newVersion *semver.Version, oldVersion *semver.Version, templateName string) error {
	filteredNewVersion := filterVersion(newVersion)
	filteredOldVersion := filterVersion(oldVersion)
	if filteredNewVersion.LessThan(filteredOldVersion) {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "ModuleTemplate"},
			templateName, field.ErrorList{field.Invalid(
				field.NewPath("spec").Child("descriptor").
					Child("version"),
				newVersion.String(), fmt.Sprintf(
					"version of templates can never be decremented (previously %s)",
					oldVersion,
				),
			)},
		)
	}
	return nil
}

func filterVersion(version *semver.Version) *semver.Version {
	filteredVersion, _ := semver.NewVersion(fmt.Sprintf("%d.%d.%d",
		version.Major(), version.Minor(), version.Patch()))
	return filteredVersion
}
