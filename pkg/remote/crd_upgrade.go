package remote

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
		client.FieldOwner(v1beta2.OperatorName))
}

func CreateOrUpdateCRD(
	ctx context.Context, plural string, kyma *v1beta2.Kyma, runtimeClient Client, controlPlaneClient Client) (
	*v1extensions.CustomResourceDefinition, *v1extensions.CustomResourceDefinition, error) {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}
	var err error
	err = controlPlaneClient.Get(
		ctx, client.ObjectKey{
			// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
			// name changes, this also has to be adjusted here. We can think of making this configurable later
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crd,
	)

	if err != nil {
		return nil, nil, err
	}

	err = runtimeClient.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crdFromRuntime,
	)

	kcpAnnotation := v1beta2.KcpKymaCRDGenerationAnnotation
	skrAnnotation := v1beta2.SkrKymaCRDGenerationAnnotation

	if plural == v1beta2.ModuleTemplateKind.Plural() {
		kcpAnnotation = v1beta2.KcpModuleTemplateCRDGenerationAnnotation
		skrAnnotation = v1beta2.SkrModuleTemplateCRDGenerationAnnotation
	}

	if ShouldPatchRemoteCRD(crdFromRuntime, crd, kyma, kcpAnnotation, skrAnnotation, err) {
		err = PatchCRD(ctx, runtimeClient, crd)
		if err != nil {
			return nil, nil, err
		}

		err = runtimeClient.Get(
			ctx, client.ObjectKey{
				Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
			}, crdFromRuntime,
		)
		if err != nil {
			return nil, nil, err
		}
	}

	if plural == v1beta2.ModuleTemplateKind.Plural() && !crdReady(crdFromRuntime) {
		return nil, nil, ErrTemplateCRDNotReady
	}

	if err != nil {
		return nil, nil, err
	}

	return crd, crdFromRuntime, nil
}

func ShouldPatchRemoteCRD(
	runtimeCrd *v1extensions.CustomResourceDefinition, kcpCrd *v1extensions.CustomResourceDefinition,
	kyma *v1beta2.Kyma, kcpAnnotation string, skrAnnotation string, err error) bool {
	latestGeneration := strconv.FormatInt(kcpCrd.Generation, 10)
	runtimeCRDGeneration := strconv.FormatInt(runtimeCrd.Generation, 10)
	return k8serrors.IsNotFound(err) || !ContainsLatestVersion(runtimeCrd, v1beta2.GroupVersion.Version) ||
		!containsLatestCRDGeneration(kyma.Annotations[kcpAnnotation], latestGeneration) ||
		!containsLatestCRDGeneration(kyma.Annotations[skrAnnotation], runtimeCRDGeneration)
}

func ContainsLatestVersion(crdFromRuntime *v1extensions.CustomResourceDefinition, latestVersion string) bool {
	for _, version := range crdFromRuntime.Spec.Versions {
		if latestVersion == version.Name {
			return true
		}
	}
	return false
}

func containsLatestCRDGeneration(storedGeneration string, latestGeneration string) bool {
	return storedGeneration == latestGeneration
}
