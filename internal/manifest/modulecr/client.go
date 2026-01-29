package modulecr

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest/finalizer"
	"github.com/kyma-project/lifecycle-manager/internal/util/collections"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var (
	ErrNoResourceDefined           = errors.New("no resource defined in the manifest")
	ErrWaitingForModuleCRsDeletion = errors.New("waiting for module CRs deletion")
)

type Client struct {
	client.Client
}

func NewClient(client client.Client) *Client {
	return &Client{
		client,
	}
}

func (c *Client) GetDefaultCR(ctx context.Context, manifest *v1beta2.Manifest) (
	*unstructured.Unstructured,
	error,
) {
	if manifest.Spec.Resource == nil || manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return nil, ErrNoResourceDefined
	}

	resourceCR := &unstructured.Unstructured{}
	name := manifest.Spec.Resource.GetName()
	namespace := manifest.Spec.Resource.GetNamespace()
	gvk := manifest.Spec.Resource.GroupVersionKind()

	resourceCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	if err := c.Get(ctx,
		client.ObjectKey{Name: name, Namespace: namespace}, resourceCR); err != nil {
		return nil, fmt.Errorf("%w: failed to fetch default resource CR", err)
	}

	return resourceCR, nil
}

func (c *Client) CheckDefaultCRDeletion(ctx context.Context, manifestCR *v1beta2.Manifest) (
	bool,
	error,
) {
	if manifestCR.Spec.Resource == nil || manifestCR.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return true, nil
	}

	defaultModuleCR := manifestCR.Spec.Resource
	moduleCRGvk := manifestCR.Spec.Resource.GroupVersionKind()
	allModuleCRs, err := c.listResourcesByGroupKindInNamespace(ctx, moduleCRGvk, defaultModuleCR.GetNamespace())
	if err != nil {
		return false, fmt.Errorf("failed to list Module CRs by group kind: %w", err)
	}

	return noDefaultModuleCRExists(allModuleCRs, defaultModuleCR), nil
}

func noDefaultModuleCRExists(allResourcesWithModuleCRGroupKind []unstructured.Unstructured,
	defaultModuleCR *unstructured.Unstructured,
) bool {
	moduleCRGvk := defaultModuleCR.GroupVersionKind()
	for _, item := range allResourcesWithModuleCRGroupKind {
		if item.GetName() == defaultModuleCR.GetName() && item.GetNamespace() == defaultModuleCR.GetNamespace() &&
			item.GroupVersionKind().Group == moduleCRGvk.Group && item.GroupVersionKind().Kind == moduleCRGvk.Kind {
			return true
		}
	}
	return false
}

func (c *Client) CheckModuleCRsDeletion(ctx context.Context, manifestCR *v1beta2.Manifest) error {
	moduleCRs, err := c.GetAllModuleCRsExcludingDefaultCR(ctx, manifestCR)
	if err != nil {
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to fetch module CRs, %w", err)
	}

	if len(moduleCRs) == 0 {
		return nil
	}

	return ErrWaitingForModuleCRsDeletion
}

// RemoveDefaultModuleCR deletes the default module CR if available in the cluster.
// It uses DeletePropagationBackground to delete module CR.
// Only if module CR is not found (indicated by NotFound error), it continues to remove Manifest finalizer,
// and we consider the CR removal successful.
func (c *Client) RemoveDefaultModuleCR(ctx context.Context, kcp client.Client, manifest *v1beta2.Manifest) error {
	crDeleted, err := c.deleteCR(ctx, manifest)
	if err != nil {
		manifest.SetStatus(manifest.GetStatus().WithErr(err))
		return err
	}
	if crDeleted {
		if err := finalizer.RemoveCRFinalizer(ctx, kcp, manifest); err != nil {
			manifest.SetStatus(manifest.GetStatus().WithErr(err))
			return err
		}
	}
	return nil
}

// SyncDefaultModuleCR sync the manifest default custom resource status in the cluster,
// if not available it created the resource.
// It is used to provide the controller with default data in the Runtime.
func (c *Client) SyncDefaultModuleCR(ctx context.Context, manifest *v1beta2.Manifest) error {
	if manifest.Spec.Resource == nil || manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	resource.SetLabels(collections.MergeMapsSilent(resource.GetLabels(), map[string]string{
		shared.ManagedBy: shared.ManagedByLabelValue,
	}))

	if err := c.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil && util.IsNotFound(err) {
		if !manifest.GetDeletionTimestamp().IsZero() {
			return nil
		}
		if err := c.Create(ctx, resource,
			client.FieldOwner(finalizer.CustomResourceManagerFinalizer)); err != nil &&
			!apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create resource: %w", err)
		}
	}
	return nil
}

func (c *Client) GetAllModuleCRsExcludingDefaultCR(ctx context.Context,
	manifest *v1beta2.Manifest,
) (
	[]unstructured.Unstructured,
	error,
) {
	if manifest.Spec.Resource == nil {
		return nil, nil
	}

	defaultModuleCR := manifest.Spec.Resource.DeepCopy()
	moduleCRGvk := defaultModuleCR.GroupVersionKind()

	allResourcesWithModuleCRGroupKind, err := c.listResourcesByGroupKindInNamespace(ctx, moduleCRGvk,
		defaultModuleCR.GetNamespace())
	if err != nil {
		return nil, fmt.Errorf("failed to list Module CRs by group kind: %w", err)
	}

	// If the CustomResourcePolicy is Ignore, we return all module CRs including the default CR
	if manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return allResourcesWithModuleCRGroupKind, nil
	}

	return filterOutDefaultCRs(allResourcesWithModuleCRGroupKind, defaultModuleCR), nil
}

func (c *Client) listResourcesByGroupKindInNamespace(ctx context.Context,
	gvk schema.GroupVersionKind,
	namespace string,
) ([]unstructured.Unstructured, error) {
	mappings, err := c.RESTMapper().RESTMappings(schema.GroupKind{
		Group: gvk.Group,
		Kind:  gvk.Kind,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get REST mappings: %w", err)
	}

	var allItems []unstructured.Unstructured
	for _, mapping := range mappings {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   mapping.GroupVersionKind.Group,
			Version: mapping.GroupVersionKind.Version,
			Kind:    mapping.GroupVersionKind.Kind,
		})

		if err := c.List(ctx, list, &client.ListOptions{
			Namespace: namespace,
		}); err != nil && !util.IsNotFound(err) {
			continue
		}

		allItems = append(allItems, list.Items...)
	}
	return allItems, nil
}

func filterOutDefaultCRs(allResourcesWithModuleCRGroupKind []unstructured.Unstructured,
	resource *unstructured.Unstructured,
) []unstructured.Unstructured {
	gvk := resource.GroupVersionKind()
	var withoutDefaultCR []unstructured.Unstructured
	for _, item := range allResourcesWithModuleCRGroupKind {
		if item.GetName() != resource.GetName() || item.GetNamespace() != resource.GetNamespace() ||
			item.GroupVersionKind().Group != gvk.Group || item.GroupVersionKind().Kind != gvk.Kind {
			withoutDefaultCR = append(withoutDefaultCR, item)
		}
	}
	return withoutDefaultCR
}

func (c *Client) deleteCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	if manifest.Spec.Resource == nil || manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return false, nil
	}

	resource := manifest.Spec.Resource.DeepCopy()
	propagation := apimetav1.DeletePropagationBackground
	err := c.Delete(ctx, resource, &client.DeleteOptions{PropagationPolicy: &propagation})
	if util.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to fetch resource: %w", err)
	}
	return false, nil
}
