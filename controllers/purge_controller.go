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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/kyma-project/lifecycle-manager/pkg/util"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/adapter"
	"github.com/kyma-project/lifecycle-manager/pkg/log"
	"github.com/kyma-project/lifecycle-manager/pkg/status"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
)

type RemoteClientResolver func(context.Context, client.ObjectKey) (client.Client, error)

type CRDMatcher func(crd apiextensions.CustomResourceDefinition) bool

// PurgeReconciler reconciles a Kyma object.
type PurgeReconciler struct {
	client.Client
	record.EventRecorder
	ResolveRemoteClient   RemoteClientResolver
	PurgeFinalizerTimeout time.Duration
	SkipCRDs              CRDMatcher
	IsManagedKyma         bool
}

func (r *PurgeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrlLog.FromContext(ctx)
	logger.V(log.InfoLevel).Info(fmt.Sprintf("Purge Reconciliation started at %s", time.Now()))

	ctx = adapter.ContextWithRecorder(ctx, r.EventRecorder)

	// check if kyma resource exists
	kyma := &v1beta2.Kyma{}
	if err := r.Get(ctx, req.NamespacedName, kyma); err != nil {
		if !util.IsNotFound(err) {
			logger.V(log.DebugLevel).Info(fmt.Sprintf("Kyma %s not found, probably already deleted", req.NamespacedName))
			return ctrl.Result{}, fmt.Errorf("purgeController: %w", err)
		}
		return ctrl.Result{}, nil
	}

	if kyma.DeletionTimestamp.IsZero() {
		if err := r.EnsurePurgeFinalizer(ctx, kyma); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// condition to check if deletionTimestamp is set, retry until it gets fully deleted
	deletionDeadline := kyma.DeletionTimestamp.Add(r.PurgeFinalizerTimeout)

	if time.Now().After(deletionDeadline) { //nolint:nestif
		remoteClient, err := r.ResolveRemoteClient(ctx, client.ObjectKeyFromObject(kyma))
		if util.IsNotFound(err) {
			if err := r.DropPurgeFinalizer(ctx, kyma); err != nil {
				logger.Error(err, "Couldn't remove Purge Finalizer from the Kyma object")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		if err != nil {
			return ctrl.Result{}, err
		}

		if err := performCleanup(ctx, remoteClient, r.SkipCRDs); err != nil {
			logger.Error(err, "Finalizer Purging failed")
			return ctrl.Result{}, err
		}

		if err := r.DropPurgeFinalizer(ctx, kyma); err != nil {
			logger.Error(err, "Couldn't remove Purge Finalizer from the Kyma object")
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}
	logger.V(log.InfoLevel).Info(fmt.Sprintf("Time until deletion deadline %s", time.Until(deletionDeadline.Add(time.Second))))

	return ctrl.Result{
		RequeueAfter: time.Until(deletionDeadline.Add(time.Second)),
	}, nil
}

func (r *PurgeReconciler) EnsurePurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.ContainsFinalizer(kyma, v1beta2.PurgeFinalizer) {
		return nil
	}

	controllerutil.AddFinalizer(kyma, v1beta2.PurgeFinalizer)
	if err := r.Update(ctx, kyma); err != nil {
		r.Event(kyma, "Warning", "SettingPurgeFinalizerError", fmt.Errorf("could not set purge finalizer: %w", err).Error())
		return fmt.Errorf("could not set purge finalizer: %w", err)
	}
	return nil
}

func (r *PurgeReconciler) DropPurgeFinalizer(ctx context.Context, kyma *v1beta2.Kyma) error {
	if controllerutil.ContainsFinalizer(kyma, v1beta2.PurgeFinalizer) {
		controllerutil.RemoveFinalizer(kyma, v1beta2.PurgeFinalizer)
		if err := r.Update(ctx, kyma); err != nil {
			r.Event(kyma, "Warning", "SettingPurgeFinalizerError",
				fmt.Errorf("could not remove purge finalizer: %w", err).Error())
			return fmt.Errorf("could not remove purge finalizer: %w", err)
		}
	}

	return nil
}

func performCleanup(ctx context.Context, remoteClient client.Client, skipCRDs CRDMatcher) error {
	crdList := apiextensions.CustomResourceDefinitionList{}

	if err := remoteClient.List(ctx, &crdList); err != nil {
		return fmt.Errorf("failed to fetch CRDs from the cluster: %w", err)
	}

	for _, crdResource := range crdList.Items {
		if isKymaCR(crdResource) {
			continue
		}

		if skipCRDs(crdResource) {
			continue
		}

		staleResources, err := getStaleResourcesFrom(ctx, remoteClient, crdResource)
		if err != nil {
			return fmt.Errorf("could not fetch stale resources from the cluster: %w", err)
		}

		err = purgeStaleResources(ctx, remoteClient, staleResources)
		if err != nil {
			return fmt.Errorf("unable to purge stale resources: %w", err)
		}
	}
	return nil
}

func getStaleResourcesFrom(ctx context.Context, remoteClient client.Client,
	crd apiextensions.CustomResourceDefinition,
) (unstructured.UnstructuredList, error) {
	staleResources := unstructured.UnstructuredList{}
	// Since there are multiple possible versions, we are choosing the one that's in the etcd storage
	var gvkVersion string
	for _, version := range crd.Spec.Versions {
		if version.Storage {
			gvkVersion = version.Name
			break
		}
	}

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Kind:    crd.Spec.Names.Kind,
		Version: gvkVersion,
	}

	staleResources.SetGroupVersionKind(gvk)

	if err := remoteClient.List(ctx, &staleResources); err != nil {
		return unstructured.UnstructuredList{}, fmt.Errorf("failed to fetch stale resources: %w", err)
	}

	return staleResources, nil
}

func purgeStaleResources(ctx context.Context, remoteClient client.Client,
	staleResources unstructured.UnstructuredList,
) error {
	for index := range staleResources.Items {
		resource := staleResources.Items[index]
		resource.SetFinalizers(nil)
		if err := remoteClient.Update(ctx, &resource); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
	}
	return nil
}

func (r *PurgeReconciler) UpdateStatus(
	ctx context.Context, kyma *v1beta2.Kyma, state v1beta2.State, message string,
) error {
	if err := status.Helper(r).UpdateStatusForExistingModules(ctx, kyma, state, message); err != nil {
		return fmt.Errorf("error while updating status to %s because of %s: %w", state, message, err)
	}
	return nil
}

func (r *PurgeReconciler) UpdateMetrics(_ context.Context, _ *v1beta2.Kyma) {}

func (r *PurgeReconciler) IsKymaManaged() bool {
	return r.IsManagedKyma
}

func isKymaCR(crd apiextensions.CustomResourceDefinition) bool {
	return crd.Spec.Group == v1beta2.GroupVersion.Group && crd.Spec.Names.Kind == string(v1beta2.KymaKind)
}

// CRDMatcherFor returns a CRDMatcher for a comma-separated list of CRDs.
// Every CRD is defined using  the syntax: `<names.plural>.<group>` or `<names.singular>.<group>`,
// e.g:. "kymas.operator.kyma-project.io" or "kyma.operator.kyma-project.io"
// Instead of a name, an asterisk `*` may be used. It matches any name for the given group.
func CRDMatcherFor(input string) CRDMatcher {
	trimmed := strings.TrimSpace(input)
	defs := strings.Split(trimmed, ",")
	if len(defs) == 0 {
		return emptyMatcher()
	}

	return crdMatcherForItems(defs)
}

func crdMatcherForItems(defs []string) CRDMatcher {
	matchers := []CRDMatcher{}
	for _, def := range defs {
		matcher := crdMatcherForItem(def)
		if matcher != nil {
			matchers = append(matchers, matcher)
		}
	}

	return func(crd apiextensions.CustomResourceDefinition) bool {
		for _, doesMatch := range matchers {
			if doesMatch(crd) {
				return true
			}
		}
		return false
	}
}

// crdMatcherForItem returns a CRDMatcher for a given CRD reference.
// The reference is expected to be in the form: `<names.plural>.<group>` or `<names.singular>.<group>`,
// e.g:. "kymas.operator.kyma-project.io" or "kyma.operator.kyma-project.io"
// Instead of a CRD name an asterisk `*` may be used, it matches any name for the given group.
// See the: k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1/CustomResourceDefinition type for details.
func crdMatcherForItem(givenCRDReference string) CRDMatcher {
	nameSegments := strings.Split(givenCRDReference, ".")
	const minSegments = 2
	if len(nameSegments) < minSegments {
		return emptyMatcher()
	}

	givenKind := strings.TrimSpace(strings.ToLower(nameSegments[0]))
	givenGroup := strings.TrimSpace(strings.ToLower(strings.Join(nameSegments[1:], ".")))

	return func(crd apiextensions.CustomResourceDefinition) bool {
		lKind := strings.ToLower(crd.Spec.Names.Kind)
		lSingular := strings.ToLower(crd.Spec.Names.Singular)
		lPlural := strings.ToLower(crd.Spec.Names.Plural)
		lGroup := strings.ToLower(crd.Spec.Group)

		if givenGroup != lGroup {
			return false
		}

		return givenKind == "*" || givenKind == lPlural || givenKind == lSingular || givenKind == lKind
	}
}

func emptyMatcher() CRDMatcher {
	return func(crd apiextensions.CustomResourceDefinition) bool {
		return false
	}
}
