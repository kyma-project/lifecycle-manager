package testutils

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrManifestResourceIsNil = errors.New("manifest spec.resource is nil")

func NewTestManifest(prefix string) *v1beta2.Manifest {
	return &v1beta2.Manifest{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", prefix, builder.RandomName()),
			Namespace: v1.NamespaceDefault,
			Labels: map[string]string{
				v1beta2.KymaName: string(uuid.NewUUID()),
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
		return nil, fmt.Errorf("get maifest %w", err)
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

func UpdateManifestState(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
	state v1beta2.State,
) error {
	component, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}
	component.Status.State = declarative.State(state)
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
