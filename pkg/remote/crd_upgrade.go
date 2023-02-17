package remote

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PatchCRD(ctx context.Context, clnt client.Client, crd *v1extensions.CustomResourceDefinition) error {
	crdToApply := &v1extensions.CustomResourceDefinition{}
	crdToApply.SetGroupVersionKind(crd.GroupVersionKind())
	crdToApply.SetName(crd.Name)
	crdToApply.Spec = crd.Spec
	crdToApply.Spec.Conversion.Strategy = v1extensions.NoneConverter
	crdToApply.Spec.Conversion.Webhook = nil
	return clnt.Patch(ctx, crdToApply,
		client.Apply,
		client.ForceOwnership,
		client.FieldOwner(v1beta1.OperatorName))
}

func ContainsLatestVersion(crdFromRuntime *v1extensions.CustomResourceDefinition, latestVersion string) bool {
	for _, version := range crdFromRuntime.Spec.Versions {
		if latestVersion == version.Name {
			return true
		}
	}
	return false
}
