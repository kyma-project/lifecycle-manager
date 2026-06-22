package crd

import (
	"context"
	"fmt"
	"reflect"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/common/fieldowners"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

type SkrClientRetrieverFunc func(kymaName types.NamespacedName) (client.Client, error)

type Repository struct {
	getSkrClient SkrClientRetrieverFunc
	crdName      string
}

func NewRepository(getSkrClient SkrClientRetrieverFunc,
	crdName string,
) *Repository {
	return &Repository{
		getSkrClient: getSkrClient,
		crdName:      crdName,
	}
}

func (r *Repository) Exists(ctx context.Context, kymaName types.NamespacedName) (bool, error) {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return false, err
	}

	err = skrClient.Get(ctx,
		types.NamespacedName{
			Name: r.crdName,
		},
		&v1beta1.PartialObjectMetadata{
			TypeMeta: apimetav1.TypeMeta{
				Kind:       reflect.TypeFor[apiextensionsv1.CustomResourceDefinition]().Name(),
				APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			},
		},
	)

	// not found error => (false, nil)
	// other error => (true, err)
	// no error => (true, nil)
	return !util.IsNotFound(err), util.IgnoreNotFound(err)
}

func (r *Repository) Delete(ctx context.Context, kymaName types.NamespacedName) error {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return err
	}

	return util.IgnoreNotFound(
		skrClient.Delete(ctx,
			&apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: apimetav1.ObjectMeta{
					Name: r.crdName,
				},
			}))
}

// Apply upserts the given CRD on the SKR cluster using Server-Side Apply.
// It uses unstructured data so that no preceding Get against the SKR is required and
// the API server resolves whether action is necessary. The conversion strategy is
// reset to None to ensure the SKR CRD does not depend on KCP-only conversion webhooks.
func (r *Repository) Apply(ctx context.Context,
	kymaName types.NamespacedName,
	kcpCrd *apiextensionsv1.CustomResourceDefinition,
) error {
	skrClient, err := r.getSkrClient(kymaName)
	if err != nil {
		return err
	}

	desired, err := buildSkrCrd(kcpCrd)
	if err != nil {
		return fmt.Errorf("failed to build SKR CRD %q: %w", r.crdName, err)
	}

	if err := skrClient.Patch(ctx, desired,
		//nolint: staticcheck // issues: #2706, #2707
		client.Apply,
		client.ForceOwnership,
		fieldowners.LegacyLifecycleManager); err != nil {
		return fmt.Errorf("failed to apply CRD %q to SKR: %w", r.crdName, err)
	}

	return nil
}

func buildSkrCrd(kcpCrd *apiextensionsv1.CustomResourceDefinition) (*unstructured.Unstructured, error) {
	spec := kcpCrd.Spec.DeepCopy()
	spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.NoneConverter,
	}

	specMap, err := machineryruntime.DefaultUnstructuredConverter.ToUnstructured(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to convert CRD spec to unstructured: %w", err)
	}

	desired := &unstructured.Unstructured{}
	desired.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind(
		reflect.TypeFor[apiextensionsv1.CustomResourceDefinition]().Name()))
	desired.SetName(kcpCrd.Name)
	desired.SetLabels(map[string]string{shared.ManagedBy: shared.ManagedByLabelValue})
	if err := unstructured.SetNestedField(desired.Object, specMap, "spec"); err != nil {
		return nil, fmt.Errorf("failed to set CRD spec on unstructured object: %w", err)
	}

	return desired, nil
}
