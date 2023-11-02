package manifest

import (
	"context"
	"errors"
	"fmt"
	"strings"

	lfLog "github.com/kyma-project/lifecycle-manager/pkg/log"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

var (
	ErrWaitingForAsyncCustomResourceDeletion = errors.New(
		"deletion of custom resource was triggered and is now waiting to be completed")
	ErrWaitingForAsyncCustomResourceDefinitionDeletion = errors.New(
		"deletion of custom resource definition was triggered and is now waiting to be completed")
)

// PostRunCreateCR is a hook for creating the manifest default custom resource if not available in the cluster
// It is used to provide the controller with default data in the Runtime.
func PostRunCreateCR(
	ctx context.Context, skr declarative.Client, kcp client.Client, obj declarative.Object,
) error {
	logger := log.FromContext(ctx)

	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}

	if !manifest.GetDeletionTimestamp().IsZero() {
		logger.V(lfLog.DebugLevel).Info("stop create resource CR for manifest under delete")
		return nil
	}
	logger.V(lfLog.DebugLevel).Info("create resource CR")

	resource := manifest.Spec.Resource.DeepCopy()
	err := skr.Create(ctx, resource, client.FieldOwner(declarative.CustomResourceManager))
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	oMeta := &v1.PartialObjectMetadata{}
	oMeta.SetName(obj.GetName())
	oMeta.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	oMeta.SetNamespace(obj.GetNamespace())
	oMeta.SetFinalizers(obj.GetFinalizers())
	if added := controllerutil.AddFinalizer(oMeta, declarative.CustomResourceManager); added {
		if err := kcp.Patch(
			ctx, oMeta, client.Apply, client.ForceOwnership, client.FieldOwner(declarative.CustomResourceManager),
		); err != nil {
			return fmt.Errorf("failed to patch resource: %w", err)
		}
		return declarative.ErrRequeueRequired
	}
	return nil
}

// PreDeleteDeleteCR is a hook for deleting the manifest default custom resource if available in the cluster
// It is used to clean up the controller default data.
// It uses DeletePropagationBackground as it will return an error if the resource exists, even if deletion is triggered
// This leads to the reconciled resource immediately being requeued due to ErrWaitingForAsyncCustomResourceDeletion.
// In this case, the next time it will run into this delete function,
// it will either say that the resource is already being deleted (2xx) and retry or its no longer found.
// Then the finalizer is dropped, and we consider the CR removal successful.
func PreDeleteDeleteCR(
	ctx context.Context, skr declarative.Client, kcp client.Client, obj declarative.Object,
) error {
	logger := log.FromContext(ctx)
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil
	}
	if manifest.Spec.Resource == nil {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	propagation := v1.DeletePropagationBackground
	err := skr.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})
	logger.V(lfLog.DebugLevel).Error(err, "resource CR delete error")

	if !util.IsNotFound(err) {
		return nil
	}

	var crd unstructured.Unstructured
	crd.SetName(GetModuleCRDName(obj))
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Version: "v1",
		Group:   "apiextensions.k8s.io",
		Kind:    "CustomResourceDefinition",
	})
	crdCopy := crd.DeepCopy()
	err = skr.Delete(ctx, crdCopy, &client.DeleteOptions{PropagationPolicy: &propagation})
	logger.V(lfLog.DebugLevel).Error(err, "resource CRD delete error")

	if !util.IsNotFound(err) {
		return nil
	}

	onCluster := manifest.DeepCopy()
	err = kcp.Get(ctx, client.ObjectKeyFromObject(obj), onCluster)
	if util.IsNotFound(err) {
		return fmt.Errorf("PreDeleteDeleteCR: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to fetch resource: %w", err)
	}
	logger.V(lfLog.DebugLevel).Info("remove CustomResourceManager finalizer")
	if removed := controllerutil.RemoveFinalizer(onCluster, declarative.CustomResourceManager); removed {
		if err := kcp.Update(
			ctx, onCluster, client.FieldOwner(declarative.CustomResourceManager),
		); err != nil {
			return fmt.Errorf("failed to update resource: %w", err)
		}
		return declarative.ErrRequeueRequired
	}
	return nil
}

func GetModuleCRDName(obj declarative.Object) string {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return ""
	}
	if manifest.Spec.Resource == nil {
		return ""
	}

	group := manifest.Spec.Resource.GroupVersionKind().Group
	name := manifest.Spec.Resource.GroupVersionKind().Kind
	return fmt.Sprintf("%s.%s", getPlural(name), group)
}

func getPlural(moduleName string) string {
	return strings.ToLower(moduleName) + "s"
}
