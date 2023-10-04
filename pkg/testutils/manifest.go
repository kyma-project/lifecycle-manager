package testutils

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	kyma *v1beta2.Kyma,
	module v1beta2.Module,
) (*v1beta2.Manifest, error) {
	template, err := ModuleTemplateFactory(module, unstructured.Unstructured{}, false, false, false, false)
	if err != nil {
		return nil, err
	}
	descriptor, err := template.GetDescriptor()
	if err != nil {
		return nil, fmt.Errorf("component.descriptor %w", err)
	}
	manifest := &v1beta2.Manifest{}
	err = clnt.Get(
		ctx, client.ObjectKey{
			Namespace: kyma.Namespace,
			Name:      common.CreateModuleName(descriptor.GetName(), kyma.Name, module.Name),
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
	kyma *v1beta2.Kyma,
	module v1beta2.Module,
) (bool, error) {
	manifest, err := GetManifest(ctx, clnt, kyma, module)
	if err != nil {
		return false, err
	}
	return manifest.Spec.Remote, nil
}

func ManifestExists(ctx context.Context,
	kyma *v1beta2.Kyma, module v1beta2.Module, controlPlaneClient client.Client,
) error {
	manifest, err := GetManifest(ctx, controlPlaneClient, kyma, module)
	return CRExists(manifest, err)
}

func UpdateManifestState(
	ctx context.Context, clnt client.Client, kyma *v1beta2.Kyma, module v1beta2.Module, state v1beta2.State,
) error {
	kyma, err := GetKyma(ctx, clnt, kyma.GetName(), kyma.GetNamespace())
	if err != nil {
		return err
	}
	component, err := GetManifest(ctx, clnt, kyma, module)
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
