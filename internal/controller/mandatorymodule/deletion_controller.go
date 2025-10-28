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
	"github.com/kyma-project/lifecycle-manager/internal/descriptor/types/ocmidentity"
	"github.com/kyma-project/lifecycle-manager/internal/event"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/queue"
	"github.com/kyma-project/lifecycle-manager/pkg/templatelookup"
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
	if err := r.Get(ctx, req.NamespacedName, template); err != nil {
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

	mrm, err := r.GetModuleReleaseMeta(ctx, template.Spec.ModuleName, template.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find ModuleReleaseMeta for Mandatory Module %s: %w",
			template.Name, err)
	}
	ocmId, err := ocmidentity.NewComponentId(mrm.Spec.OcmComponentName, template.Spec.Version)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create OCM identity for Mandatory Module %s: %w",
			template.Spec.ModuleName, err)
	}

	manifests, err := r.getCorrespondingManifests(ctx, template.Namespace, *ocmId)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get MandatoryModuleManifests: %w", err)
	}

	if len(manifests) == 0 {
		if controllerutil.RemoveFinalizer(template, shared.MandatoryModuleFinalizer) {
			return r.updateTemplateFinalizer(ctx, template)
		}
		return ctrl.Result{}, nil
	}

	if err := r.removeManifests(ctx, manifests); err != nil {
		r.Event.Warning(template, deletingManifestError, err)
		return ctrl.Result{}, fmt.Errorf("failed to remove MandatoryModule Manifest: %w", err)
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *DeletionReconciler) GetModuleReleaseMeta(ctx context.Context, moduleName, namespace string) (
	*v1beta2.ModuleReleaseMeta, error,
) {
	return templatelookup.GetModuleReleaseMeta(ctx, r.Client, moduleName, namespace)
}

func (r *DeletionReconciler) updateTemplateFinalizer(ctx context.Context,
	template *v1beta2.ModuleTemplate,
) (ctrl.Result, error) {
	if err := r.Update(ctx, template); err != nil {
		r.Event.Warning(template, settingFinalizerError, err)
		return ctrl.Result{}, fmt.Errorf("failed to update MandatoryModuleTemplate finalizer: %w", err)
	}
	return ctrl.Result{Requeue: true}, nil
}

func (r *DeletionReconciler) getCorrespondingManifests(ctx context.Context,
	namespace string, ocmId ocmidentity.ComponentId) ([]v1beta2.Manifest,
	error,
) {
	manifests := &v1beta2.ManifestList{}
	if err := r.List(ctx, manifests, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: k8slabels.SelectorFromSet(k8slabels.Set{shared.IsMandatoryModule: "true"}),
	}); client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("not able to list mandatory module manifests: %w", err)
	}

	filtered := filterManifestsByComponentIdentity(manifests.Items, ocmId)

	return filtered, nil
}

func (r *DeletionReconciler) removeManifests(ctx context.Context, manifests []v1beta2.Manifest) error {
	for _, manifest := range manifests {
		if err := r.Delete(ctx, &manifest); err != nil {
			return fmt.Errorf("not able to delete manifest %s/%s: %w", manifest.Namespace, manifest.Name, err)
		}
	}
	logf.FromContext(ctx).V(log.DebugLevel).Info("Marked all MandatoryModule Manifests for deletion")
	return nil
}

// filterManifestsByComponentIdentity filters the manifests by OCM Component Name and module version.
// OCM Component Name is a fully qualified name that looks like: 'kyma-project.io/module/<module-name>'.
func filterManifestsByComponentIdentity(manifests []v1beta2.Manifest,
	ocmId ocmidentity.ComponentId,
) []v1beta2.Manifest {
	filteredManifests := make([]v1beta2.Manifest, 0)
	for _, manifest := range manifests {
		if manifest.Annotations == nil {
			continue
		}

		if manifest.Annotations[shared.FQDN] == ocmId.Name() && manifest.Spec.Version == ocmId.Version() {
			filteredManifests = append(filteredManifests, manifest)
		}
	}
	return filteredManifests
}
