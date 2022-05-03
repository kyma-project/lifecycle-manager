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
	"errors"
	"fmt"
	"golang.org/x/mod/semver"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"strings"
	"time"
)

// log is for logging in this package.
var kymalog = logf.Log.WithName("kyma-resource")

func (r *Kyma) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-operator-kyma-project-io-v1alpha1-kyma,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=kymas,verbs=create;update,versions=v1alpha1,name=mkyma.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Kyma{}

const PrefixReleaseRevision = "release.kyma-project.io"
const RevisionHistoryLimitDefault = 3

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Kyma) Default() {
	// if our active release is also the specified release, we should back up the release spec
	jsonout, err := json.Marshal(r.Spec)
	if err != nil {
		return
	}

	revision := fmt.Sprintf("%s/%s-%s", PrefixReleaseRevision, r.Spec.Release, time.Now().Format("2006-01-02T15.04.05.000"))

	if r.Spec.RevisionHistoryLimit <= 0 {
		r.Spec.RevisionHistoryLimit = RevisionHistoryLimitDefault
	}

	var observedrevisions = r.Spec.RevisionHistoryLimit
	for rev := range r.Annotations {
		if strings.HasPrefix(rev, PrefixReleaseRevision) {
			observedrevisions--
			if observedrevisions <= 0 {
				delete(r.Annotations, rev)
			}
		}
	}

	r.Annotations[revision] = string(jsonout)
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-operator-kyma-project-io-v1alpha1-kyma,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.kyma-project.io,resources=kymas,verbs=create;update,versions=v1alpha1,name=vkyma.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Kyma{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Kyma) ValidateCreate() error {
	kymalog.Info("validate create", "name", r.Name)
	return r.ValidateSemVer()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Kyma) ValidateUpdate(old runtime.Object) error {
	kymalog.Info("validate update", "name", r.Name)
	return r.ValidateSemVer()
}

func (r *Kyma) ValidateSemVer() error {
	if !semver.IsValid(r.Spec.Release) {
		return errors.New(fmt.Sprintf("reconciliation of %s is not possible, invalid semver!", r.Name))
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Kyma) ValidateDelete() error {
	kymalog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
