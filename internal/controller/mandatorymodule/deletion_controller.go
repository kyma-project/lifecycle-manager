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

package mandatorymodule

import (
	"context"
	"fmt"

	k8slabels "k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/provider"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	settingFinalizerError event.Reason = "SettingMandatoryModuleTemplateFinalizerError"
	deletingManifestError event.Reason = "DeletingMandatoryModuleManifestError"
)

type DeletionReconciler struct {
	client.Client
	event.Event
	queue.RequeueIntervals

	DescriptorProvider *provider.CachedDescriptorProvider
}

func (r *DeletionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(log.DebugLevel).Info("Mandatory Module Deletion Reconciliation started")

	template := &v1beta2.ModuleTemplate{}
	err := r.Get(ctx, req.NamespacedName, template)
	if err != nil {
		if util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("ModuleTemplate %s not found, probably already deleted",
				req.NamespacedName))
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("MandatoryModuleDeletionController: %w", err)
	}

	if !template.Spec.Mandatory {
		logger.V(log.DebugLevel).Info("Skipping reconciliation, ModuleTemplate not marked as mandatory")
		return ctrl.Result{}, nil
	}

	updateRequired := controllerutil.AddFinalizer(template, shared.MandatoryModuleFinalizer)
	if updateRequired {
		return r.updateTemplateFinalizer(ctx, template)
	}

	if template.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	manifests, err := r.getCorrespondingManifests(ctx, template)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get MandatoryModuleManifests: %w", err)
	}

	if len(manifests) == 0 {
		if controllerutil.RemoveFinalizer(template, shared.MandatoryModuleFinalizer) {
			return r.updateTemplateFinalizer(ctx, template)
		}
		return ctrl.Result{}, nil
	}

	err = r.removeManifests(ctx, manifests)
	if err != nil {
		r.Event.Warning(template, deletingManifestError, err)
		return ctrl.Result{}, fmt.Errorf("failed to remove MandatoryModule Manifest: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *DeletionReconciler) updateTemplateFinalizer(ctx context.Context,
	template *v1beta2.ModuleTemplate,
) (ctrl.Result, error) {
	err := r.Update(ctx, template)
	if err != nil {
		r.Event.Warning(template, settingFinalizerError, err)
		return ctrl.Result{}, fmt.Errorf("failed to update MandatoryModuleTemplate finalizer: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *DeletionReconciler) getCorrespondingManifests(ctx context.Context,
	template *v1beta2.ModuleTemplate) ([]v1beta2.Manifest,
	error,
) {
	manifests := &v1beta2.ManifestList{}
	descriptor, err := r.DescriptorProvider.GetDescriptor(template)
	if err != nil {
		return nil, fmt.Errorf("not able to get descriptor from template: %w", err)
	}
	err = r.List(ctx, manifests, &client.ListOptions{
		Namespace:     template.Namespace,
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: "true"}),
	})
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("not able to list mandatory module manifests: %w", err)
	}

	filtered := filterManifestsByFQDNAndVersion(manifests.Items, descriptor.GetName(), descriptor.GetVersion())

	return filtered, nil
}

func (r *DeletionReconciler) removeManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for _, manifest := range manifests {
		err := r.Delete(ctx, &manifest)
		if err != nil {
			return fmt.Errorf("not able to delete manifest %s/%s: %w", manifest.Namespace, manifest.Name, err)
		}
	}
	logf.FromContext(ctx).V(log.DebugLevel).Info("Marked all MandatoryModule Manifests for deletion")
	return nil
}

func filterManifestsByFQDNAndVersion(manifests []v1beta2.Manifest,
	fqdn, moduleVersion string,
) []v1beta2.Manifest {
	filteredManifests := make([]v1beta2.Manifest, 0)
	for _, manifest := range manifests {
		if manifest.Annotations == nil {
			continue
		}

		if manifest.Annotations[shared.FQDN] == fqdn && manifest.Spec.Version == moduleVersion {
			filteredManifests = append(filteredManifests, manifest)
		}
	}
	return filteredManifests
}
