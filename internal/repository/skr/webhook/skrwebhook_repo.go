package webhook

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"golang.org/x/sync/errgroup"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	skrwebhookresources "github.com/kyma-project/lifecycle-manager/internal/service/watcher/resources"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientRetrieverFunc func(kymaName types.NamespacedName) (client.Client, error)

type ResourceRepository struct {
	resources           []v1beta1.PartialObjectMetadata
	getSkrClient        SkrClientRetrieverFunc
	remoteSyncNamespace string
}

const numDynamicResources = 2

func NewResourceRepository(
	getSkrClient SkrClientRetrieverFunc,
	remoteSyncNamespace string,
	baseResources []*unstructured.Unstructured,
) *ResourceRepository {
	resources := make([]v1beta1.PartialObjectMetadata, 0, len(baseResources)+numDynamicResources)
	for _, res := range baseResources {
		meta := v1beta1.PartialObjectMetadata{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       res.GetKind(),
				APIVersion: res.GetAPIVersion(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      res.GetName(),
				Namespace: remoteSyncNamespace,
			},
		}
		resources = append(resources, meta)
	}

	//nolint:revive // false positive ¯\_(ツ)_/¯
	// Add generated resources that are created dynamically from SkrWebhookManifestManager (not read from resources.yaml)
	resources = append(resources,
		v1beta1.PartialObjectMetadata{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       reflect.TypeOf(admissionregistrationv1.ValidatingWebhookConfiguration{}).Name(),
				APIVersion: admissionregistrationv1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      skrwebhookresources.SkrResourceName,
				Namespace: remoteSyncNamespace,
			},
		},
		v1beta1.PartialObjectMetadata{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       reflect.TypeOf(apicorev1.Secret{}).Name(),
				APIVersion: apicorev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: apimetav1.ObjectMeta{
				Name:      skrwebhookresources.SkrTLSName,
				Namespace: remoteSyncNamespace,
			},
		},
	)

	return &ResourceRepository{
		resources:           resources,
		getSkrClient:        getSkrClient,
		remoteSyncNamespace: remoteSyncNamespace,
	}
}

// ResourcesExist checks if any of the skr-webhook resources exist in the SKR cluster for the given Kyma.
func (r *ResourceRepository) ResourcesExist(ctx context.Context, kymaName types.NamespacedName) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return false, err
	}

	errGrp, grpCtx := errgroup.WithContext(ctx)
	resourceExists := make(chan bool, 1)

	for resIdx := range r.resources {
		errGrp.Go(func() error {
			select {
			case <-grpCtx.Done():
				// Short-circuit if context is cancelled (resource already found)
				return nil
			default:
			}

			ref := r.resources[resIdx]
			err := skrClient.Get(grpCtx, client.ObjectKeyFromObject(&ref), &ref)
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Resource does not exist, continue checking others
					return nil
				}

				if errors.Is(err, context.Canceled) {
					// Operation was canceled, likely because a resource was found
					return nil
				}

				return fmt.Errorf("resource name %s: %w", ref.Name, err)
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
		return false, fmt.Errorf("failed to check resource: %w", err)
	}

	select {
	case <-resourceExists:
		return true, nil
	default:
		return false, nil
	}
}

// DeleteWebhookResources deletes all skr-webhook resources from the SKR cluster for the given Kyma.
func (r *ResourceRepository) DeleteWebhookResources(ctx context.Context, kymaName types.NamespacedName) error {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return err
	}

	errGrp, grpCtx := errgroup.WithContext(ctx)

	for resIdx := range r.resources {
		errGrp.Go(func() error {
			ref := r.resources[resIdx]
			err := skrClient.Delete(grpCtx, &ref)
			if err != nil && !util.IsNotFound(err) {
				return fmt.Errorf("failed to delete resource %s: %w", ref.Name, err)
			}
			return nil
		})
	}

	if err := errGrp.Wait(); err != nil {
		return fmt.Errorf("failed to delete webhook resources: %w", err)
	}

	return nil
}
