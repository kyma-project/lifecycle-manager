package webhook

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientCache interface {
	Get(key client.ObjectKey) client.Client
}

type ResourceRepository struct {
	resources           []*unstructured.Unstructured
	skrClientCache      SkrClientCache
	remoteSyncNamespace string
}

func NewResourceRepository(
	webhookResources []*unstructured.Unstructured,
	skrClientCache SkrClientCache,
	remoteSyncNamespace string,
) *ResourceRepository {
	return &ResourceRepository{
		resources:           webhookResources,
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

			resource := r.resources[resIdx].DeepCopy()
			resource.SetNamespace(r.remoteSyncNamespace)

			err := skrClient.Get(grpCtx, client.ObjectKeyFromObject(resource), resource)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Resource does not exist - continue checking others
					return nil
				}
				return fmt.Errorf("failed to check resource %s: %w", resource.GetName(), err)
			}
			// Resource exists - signal and cancel context to stop other checks
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
			resource := r.resources[resIdx].DeepCopy()
			resource.SetNamespace(r.remoteSyncNamespace)

			err := skrClient.Delete(grpCtx, resource)
			if err != nil {
				return fmt.Errorf("failed to delete resource %s: %w", resource.GetName(), err)
			}
			return nil
		})
	}

	if err := errGrp.Wait(); err != nil && !util.IsNotFound(err) {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}

	return nil
}
