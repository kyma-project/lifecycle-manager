package testutils

import (
	"context"
	"errors"
	"fmt"

	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
)

var (
	ErrManifestResourceIsNil = errors.New("manifest spec.resource is nil")
	ErrManifestsExist        = errors.New("cluster contains manifest CRs")
)

func NewTestManifest(prefix string) *v1beta2.Manifest {
	return &v1beta2.Manifest{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", prefix, builder.RandomName()),
			Namespace: apimetav1.NamespaceDefault,
			Labels: map[string]string{
				shared.KymaName: string(uuid.NewUUID()),
			},
			Annotations: map[string]string{},
		},
	}
}

func GetManifest(ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) (*v1beta2.Manifest, error) {
	kyma, err := GetKyma(ctx, clnt, kymaName, kymaNamespace)
	if err != nil {
		return nil, err
	}

	var manifestKey v1beta2.TrackingObject
	for _, module := range kyma.Status.Modules {
		if module.Name == moduleName {
			manifestKey = *module.Manifest
		}
	}

	manifest := &v1beta2.Manifest{}
	err = clnt.Get(
		ctx, client.ObjectKey{
			Namespace: manifestKey.Namespace,
			Name:      manifestKey.Name,
		}, manifest,
	)
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}
	return manifest, nil
}

func GetManifestSpecRemote(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) (bool, error) {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return false, err
	}
	return manifest.Spec.Remote, nil
}

func ManifestExists(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	return CRExists(manifest, err)
}

func NoManifestExist(ctx context.Context,
	clnt client.Client,
) error {
	manifestList := &v1beta2.ManifestList{}
	if err := clnt.List(ctx, manifestList); err != nil {
		return fmt.Errorf("error listing manifests: %w", err)
	}
	if len(manifestList.Items) == 0 {
		return nil
	}
	return fmt.Errorf("error checking no manifests exist on cluster: %w", ErrManifestsExist)
}

func UpdateManifestState(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
	state shared.State,
) error {
	component, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}
	component.Status.State = state
	err = clnt.Status().Update(ctx, component)
	if err != nil {
		return fmt.Errorf("update manifest: %w", err)
	}
	return nil
}

func GetManifestResource(ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) (*unstructured.Unstructured, error) {
	moduleInCluster, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return nil, err
	}
	if moduleInCluster.Spec.Resource == nil {
		return nil, ErrManifestResourceIsNil
	}

	return moduleInCluster.Spec.Resource, nil
}

func AddSkipLabelToManifest(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return fmt.Errorf("failed to get manifest, %w", err)
	}

	manifest.Labels[shared.SkipReconcileLabel] = "true"
	err = clnt.Update(ctx, manifest)
	if err != nil {
		return fmt.Errorf("failed to update manifest, %w", err)
	}

	return nil
}

func SkipLabelExistsInManifest(ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) bool {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return false
	}

	return manifest.Labels[shared.SkipReconcileLabel] == "true"
}
