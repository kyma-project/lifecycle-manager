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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var manifestlog = logf.Log.WithName("manifest-resource") //nolint:gochecknoglobals

func (m *Manifest) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

//nolint:lll
//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1alpha1-manifest,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=manifests,verbs=create;update,versions=v1alpha1,name=v1alpha1.mmanifest.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Manifest{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (m *Manifest) Default() {
	var emptyImageSpec ImageSpec
	if m.Spec.Config == emptyImageSpec {
		m.Spec.Config = ImageSpec{}
	}

	if m.Spec.Installs == nil {
		m.Spec.Installs = make([]InstallInfo, 0)
	}
}

//nolint:lll
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1alpha1-manifest,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=manifests,verbs=create;update,versions=v1alpha1,name=v1alpha1.vmanifest.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Manifest{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *Manifest) ValidateCreate() error {
	manifestlog.Info("validate create", "name", m.Name)

	return m.validateInstalls()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *Manifest) ValidateUpdate(old runtime.Object) error {
	manifestlog.Info("validate update", "name", m.Name)

	return m.validateInstalls()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *Manifest) ValidateDelete() error {
	manifestlog.Info("validate delete", "name", m.Name)
	return nil
}

func (m *Manifest) validateInstalls() error {
	fieldErrors := ValidateInstalls(m)

	if len(fieldErrors) > 0 {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: ManifestKind}, m.Name, fieldErrors,
		)
	}

	return nil
}
