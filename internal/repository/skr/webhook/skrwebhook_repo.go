package webhook

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientCache interface {
	Get(key client.ObjectKey) client.Client
}

// ResourceRef contains the minimal information needed to identify and delete a resource.
type ResourceRef struct {
	Name       string
	APIVersion string
	Kind       string
}

type ResourceRepository struct {
	resources           []ResourceRef
	skrClientCache      SkrClientCache
	remoteSyncNamespace string
}

func NewResourceRepository(
	skrClientCache SkrClientCache,
	remoteSyncNamespace string,
	baseResources []*unstructured.Unstructured,
) *ResourceRepository {
	resources := make([]ResourceRef, 0, len(baseResources)+2)
	for _, res := range baseResources {
		gvk := res.GroupVersionKind()

		resources = append(resources, ResourceRef{
			Name:       res.GetName(),
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
		})
	}

	// Add generated resources that are created dynamically from SkrWebhookManifestManager (not read from resources.yaml)
	resources = append(resources,
		ResourceRef{
			Name:       skrwebhookresources.SkrResourceName,
			APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			Kind:       "ValidatingWebhookConfiguration",
		},
		ResourceRef{
			Name:       skrwebhookresources.SkrTLSName,
			APIVersion: "v1",
			Kind:       "Secret",
		},
	)

	return &ResourceRepository{
		resources:           resources,
		skrClientCache:      skrClientCache,
		remoteSyncNamespace: remoteSyncNamespace,
	}
}

// ResourcesExist checks if any of the skr-webhook resources exist in the SKR cluster for the given Kyma.
func (r *ResourceRepository) ResourcesExist(kymaName string) (bool, error) {
	kymaNamespacedName := types.NamespacedName{
		Name:      kymaName,
		Namespace: r.remoteSyncNamespace,
	}

	skrClient := r.skrClientCache.Get(kymaNamespacedName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errGrp, grpCtx := errgroup.WithContext(ctx)
	resourceExists := make(chan bool, 1)

	for idx := range r.resources {
		resIdx := idx
		errGrp.Go(func() error {
			select {
			case <-grpCtx.Done():
				// Short-circuit if context is cancelled (resource already found)
				return nil
			default:
			}

			ref := r.resources[resIdx]
			resource := &unstructured.Unstructured{}
			resource.SetGroupVersionKind(schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind))
			resource.SetName(ref.Name)
			resource.SetNamespace(r.remoteSyncNamespace)

			err := skrClient.Get(grpCtx, client.ObjectKeyFromObject(resource), resource)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Resource does not exist, continue checking others
					return nil
				}
				return fmt.Errorf("failed to check resource %s: %w", ref.Name, err)
			}
			select {
			case resourceExists <- true:
				cancel()
			default:
			}
			return nil
		})
	}

	if err := errGrp.Wait(); err != nil {
		return false, err
	}

	select {
	case <-resourceExists:
		return true, nil
	default:
		return false, nil
	}
}

// DeleteWebhookResources deletes all skr-webhook resources from the SKR cluster for the given Kyma.
func (r *ResourceRepository) DeleteWebhookResources(ctx context.Context, kymaName string) error {
	kymaNamespacedName := types.NamespacedName{
		Name:      kymaName,
		Namespace: r.remoteSyncNamespace,
	}

	skrClient := r.skrClientCache.Get(kymaNamespacedName)

	errGrp, grpCtx := errgroup.WithContext(ctx)

	for idx := range r.resources {
		resIdx := idx
		errGrp.Go(func() error {
			ref := r.resources[resIdx]
			resource := &unstructured.Unstructured{}
			resource.SetGroupVersionKind(schema.FromAPIVersionAndKind(ref.APIVersion, ref.Kind))
			resource.SetName(ref.Name)
			resource.SetNamespace(r.remoteSyncNamespace)

			err := skrClient.Delete(grpCtx, resource)
			if err != nil {
				return fmt.Errorf("failed to delete resource %s: %w", ref.Name, err)
			}
			return nil
		})
	}

	if err := errGrp.Wait(); err != nil && !util.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}

	return nil
}
