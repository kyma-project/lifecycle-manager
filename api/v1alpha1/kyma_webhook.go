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
	"github.com/kyma-project/lifecycle-manager/pkg/channel"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var kymalog = logf.Log.WithName("kyma-resource")

type kymaValidator struct {
	Client client.Client
}

func (kyma *Kyma) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		WithValidator(&kymaValidator{Client: mgr.GetClient()}).
		For(kyma).
		Complete()
}

//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1alpha1-kyma,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=kymas,verbs=create;update,versions=v1alpha1,name=vkyma.kb.io,admissionReviewVersions=v1

// ValidateCreate implements admission.CustomValidator interface
func (v *kymaValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	kyma := obj.(*Kyma)
	kymalog.Info("validate create", "name", kyma.Name)
	return v.validate(ctx, nil, kyma)
}

// ValidateUpdate implements admission.CustomValidator interface
func (v *kymaValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) error {
	newKyma := newObj.(*Kyma)
	moduletemplatelog.Info("validate update", "name", newKyma.Name)
	return v.validate(ctx, nil, newKyma)
}

// ValidateDelete implements admission.CustomValidator interface
func (v *kymaValidator) ValidateDelete(_ context.Context, _ runtime.Object) error {
	return nil
}

func (v *kymaValidator) validate(ctx context.Context, _, new *Kyma) error {
	_, err := channel.GetTemplates(ctx, v.Client, new)

	if err != nil {
		return apierrors.NewInvalid(
			schema.GroupKind{Group: GroupVersion.Group, Kind: "Kyma"},
			new.Name, field.ErrorList{field.Invalid(field.NewPath("spec").Child("modules"),
				string(new.Spec.Modules), err.Error())})
	}
	return nil
}
