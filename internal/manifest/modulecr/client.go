package modulecr

import (
	"context"
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
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
	allModuleCRs, err := c.listResourcesByGroupKindInAllNamespaces(ctx, defaultModuleCR.GroupVersionKind().GroupKind())
	if util.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to list Module CRs by group kind: %w", err)
	}

	return c.noDefaultModuleCRExists(allModuleCRs, defaultModuleCR), nil
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
			fieldowners.CustomResourceFinalizer); err != nil &&
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

	allResourcesWithModuleCRGroupKind, err := c.listResourcesByGroupKindInAllNamespaces(ctx,
		defaultModuleCR.GroupVersionKind().GroupKind())
	if err != nil {
		return nil, fmt.Errorf("failed to list Module CRs by group kind: %w", err)
	}

	// If the CustomResourcePolicy is Ignore, we return all module CRs including the default CR
	if manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return allResourcesWithModuleCRGroupKind, nil
	}

	return c.filterOutDefaultCRs(allResourcesWithModuleCRGroupKind, defaultModuleCR), nil
}

func (c *Client) noDefaultModuleCRExists(allResourcesWithModuleCRGroupKind []unstructured.Unstructured,
	defaultModuleCR *unstructured.Unstructured,
) bool {
	for _, resource := range allResourcesWithModuleCRGroupKind {
		if c.isResourceTheDefaultCR(&resource, defaultModuleCR) {
			return false
		}
	}
	return true
}

// listResourcesByGroupKindInAllNamespaces lists all resources matching the given GroupKind
// across ALL namespaces. This is required by ADR #972 to check all Module CRs before deletion.
func (c *Client) listResourcesByGroupKindInAllNamespaces(ctx context.Context,
	gk schema.GroupKind,
) ([]unstructured.Unstructured, error) {
	mappings, err := c.RESTMapper().RESTMappings(gk)
	if err != nil {
		return nil, fmt.Errorf("failed to get REST mappings for %s.%s: %w", gk.Group, gk.Kind, err)
	}

	var allItems []unstructured.Unstructured
	for _, mapping := range mappings {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   mapping.GroupVersionKind.Group,
			Version: mapping.GroupVersionKind.Version,
			Kind:    mapping.GroupVersionKind.Kind,
		})

		// Empty ListOptions means search across ALL namespaces
		if err := c.List(ctx, list, &client.ListOptions{}); err != nil && !util.IsNotFound(err) {
			continue
		}

		allItems = append(allItems, list.Items...)
	}
	return allItems, nil
}

func (c *Client) filterOutDefaultCRs(allResourcesWithModuleCRGroupKind []unstructured.Unstructured,
	defaultModuleCR *unstructured.Unstructured,
) []unstructured.Unstructured {
	var withoutDefaultCR []unstructured.Unstructured
	for _, resource := range allResourcesWithModuleCRGroupKind {
		if !c.isResourceTheDefaultCR(&resource, defaultModuleCR) {
			withoutDefaultCR = append(withoutDefaultCR, resource)
		}
	}
	return withoutDefaultCR
}

func (c *Client) isResourceTheDefaultCR(resource *unstructured.Unstructured,
	defaultModuleCR *unstructured.Unstructured,
) bool {
	moduleCRGvk := defaultModuleCR.GroupVersionKind()

	// For cluster-scoped resources:
	// - The actual CR in the cluster has namespace=""
	// We use a defensive approach: check both the resource namespace AND query the CRD scope.
	namespacesMatch := resource.GetNamespace() == defaultModuleCR.GetNamespace()
	if resource.GetNamespace() == "" || c.isClusterScoped(resource.GroupVersionKind().GroupKind()) {
		// Resource is cluster-scoped: either has empty namespace OR CRD defines scope: Cluster
		// In this case, ignore the namespace from defaultModuleCR spec
		namespacesMatch = true
	}

	return resource.GetName() == defaultModuleCR.GetName() &&
		namespacesMatch &&
		resource.GroupVersionKind().Group == moduleCRGvk.Group &&
		resource.GroupVersionKind().Kind == moduleCRGvk.Kind
}

// isClusterScoped checks if a resource is cluster-scoped by querying the CRD scope via RESTMapper.
// Returns true if the CRD defines scope: Cluster, false if scope: Namespaced or unable to determine.
func (c *Client) isClusterScoped(gk schema.GroupKind) bool {
	mappings, err := c.RESTMapper().RESTMappings(gk)
	if err != nil {
		return false
	}

	// Check if any mapping indicates cluster scope (meta.RESTScopeNameRoot)
	for _, mapping := range mappings {
		if mapping.Scope.Name() == meta.RESTScopeNameRoot {
			return true
		}
	}

	return false
}

func (c *Client) deleteCR(ctx context.Context, manifest *v1beta2.Manifest) (bool, error) {
	if manifest.Spec.Resource == nil || manifest.Spec.CustomResourcePolicy == v1beta2.CustomResourcePolicyIgnore {
		return false, nil
	}

	defaultModuleCR := manifest.Spec.Resource.DeepCopy()

	allModuleCRs, err := c.listResourcesByGroupKindInAllNamespaces(ctx, defaultModuleCR.GroupVersionKind().GroupKind())
	if util.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to list Module CRs by group kind: %w", err)
	}

	var resourceToDelete *unstructured.Unstructured
	for _, cr := range allModuleCRs {
		if c.isResourceTheDefaultCR(&cr, defaultModuleCR) {
			resourceToDelete = &cr
			break
		}
	}

	if resourceToDelete == nil {
		return true, nil
	}

	propagation := apimetav1.DeletePropagationBackground
	err = c.Delete(ctx, resourceToDelete, &client.DeleteOptions{PropagationPolicy: &propagation})
	if util.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to delete resource: %w", err)
	}

	return false, nil
}
