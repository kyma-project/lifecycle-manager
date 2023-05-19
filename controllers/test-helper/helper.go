package helper

import (
	"context"
	"errors"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNotFound = errors.New("resource not exists")
)

func GetModuleTemplate(ctx context.Context,
	clnt client.Client, name, namespace string) (*v1beta2.ModuleTemplate, error) {
	moduleTemplateInCluster := &v1beta2.ModuleTemplate{}
	moduleTemplateInCluster.SetNamespace(namespace)
	moduleTemplateInCluster.SetName(name)
	err := clnt.Get(ctx, client.ObjectKeyFromObject(moduleTemplateInCluster), moduleTemplateInCluster)
	if err != nil {
		return nil, err
	}
	return moduleTemplateInCluster, nil
}

func ManifestExists(ctx context.Context,
	kyma *v1beta2.Kyma, module v1beta2.Module, controlPlaneClient client.Client) error {
	_, err := testutils.GetManifest(ctx, controlPlaneClient, kyma, module)
	if k8serrors.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func ModuleTemplateExists(ctx context.Context, client client.Client, name, namespace string) error {
	_, err := GetModuleTemplate(ctx, client, name, namespace)
	if k8serrors.IsNotFound(err) {
		return ErrNotFound
	}
	return nil
}

func ModuleTemplatesExist(ctx context.Context,
	clnt client.Client, kyma *v1beta2.Kyma, remoteSyncNamespace string) func() error {
	return func() error {
		for _, module := range kyma.Spec.Modules {
			if err := ModuleTemplateExists(ctx, clnt, module.Name, remoteSyncNamespace); err != nil {
				return err
			}
		}

		return nil
	}
}
