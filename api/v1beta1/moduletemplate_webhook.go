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

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"k8s.io/apimachinery/pkg/runtime"
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

func (c *clusterAwareModuleTemplateValidator) ValidateCreate(_ context.Context, obj runtime.Object) error {
	newTemplate := obj.(*ModuleTemplate)
	newDescriptor, err := newTemplate.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	moduletemplatelog.Info("validate create", "name", newTemplate.Name)
	return v1beta2.Validate(nil, newDescriptor, newTemplate.Name)
}

func (c *clusterAwareModuleTemplateValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) error {
	newTemplate := newObj.(*ModuleTemplate)
	newDescriptor, err := newTemplate.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	oldTemplate := oldObj.(*ModuleTemplate)
	oldDescriptor, err := oldTemplate.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	moduletemplatelog.Info("validate update", "name", newObj.(*ModuleTemplate).Name)
	return v1beta2.Validate(oldDescriptor, newDescriptor, newTemplate.Name)
}

func (c *clusterAwareModuleTemplateValidator) ValidateDelete(_ context.Context, obj runtime.Object) error {
	newTemplate := obj.(*ModuleTemplate)
	newDescriptor, err := newTemplate.Spec.GetDescriptor()
	if err != nil {
		return err
	}
	moduletemplatelog.Info("validate delete", "name", obj.(*ModuleTemplate).Name)
	return v1beta2.Validate(nil, newDescriptor, newTemplate.Name)
}
