package remote

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/cache"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func PatchCRD(ctx context.Context, clnt client.Client, crd *v1extensions.CustomResourceDefinition) error {
	crdToApply := &v1extensions.CustomResourceDefinition{}
	crdToApply.SetGroupVersionKind(crd.GroupVersionKind())
	crdToApply.SetName(crd.Name)
	crdToApply.Spec = crd.Spec
	crdToApply.Spec.Conversion.Strategy = v1extensions.NoneConverter
	crdToApply.Spec.Conversion.Webhook = nil
	err := clnt.Patch(ctx, crdToApply,
		client.Apply,
		client.ForceOwnership,
		client.FieldOwner(v1beta2.OperatorName))
	if err != nil {
		return fmt.Errorf("failed to patch CRD: %w", err)
	}
	return nil
}

type CrdType string

const (
	KCP CrdType = "KCP"
	SKR CrdType = "SKR"
)

func updateRemoteCRD(ctx context.Context, kyma *v1beta2.Kyma, runtimeClient Client,
	crdFromRuntime *v1extensions.CustomResourceDefinition, kcpCrd *v1extensions.CustomResourceDefinition,
) (bool, error) {
	if ShouldPatchRemoteCRD(crdFromRuntime, kcpCrd, kyma) {
		err := PatchCRD(ctx, runtimeClient, kcpCrd)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	return false, nil
}

func ShouldPatchRemoteCRD(
	runtimeCrd *v1extensions.CustomResourceDefinition, kcpCrd *v1extensions.CustomResourceDefinition,
	kyma *v1beta2.Kyma,
) bool {
	kcpAnnotation := getAnnotation(kcpCrd, KCP)
	skrAnnotation := getAnnotation(runtimeCrd, SKR)

	latestGeneration := strconv.FormatInt(kcpCrd.Generation, 10)
	runtimeCRDGeneration := strconv.FormatInt(runtimeCrd.Generation, 10)
	return kyma.Annotations[kcpAnnotation] != latestGeneration ||
		kyma.Annotations[skrAnnotation] != runtimeCRDGeneration
}

func updateKymaAnnotations(kyma *v1beta2.Kyma, crd *v1extensions.CustomResourceDefinition, crdType CrdType) {
	if kyma.Annotations == nil {
		kyma.Annotations = make(map[string]string)
	}
	annotation := getAnnotation(crd, crdType)
	kyma.Annotations[annotation] = strconv.FormatInt(crd.Generation, 10)
}

func getAnnotation(crd *v1extensions.CustomResourceDefinition, crdType CrdType) string {
	return fmt.Sprintf("%s-%s-crd-generation", strings.ToLower(crd.Spec.Names.Kind), strings.ToLower(string(crdType)))
}

func SyncCrdsAndUpdateKymaAnnotations(ctx context.Context, kyma *v1beta2.Kyma,
	runtimeClient Client, controlPlaneClient Client,
) (bool, error) {
	kymaCrdUpdated, err := fetchCrdsAndUpdateKymaAnnotations(ctx, controlPlaneClient,
		runtimeClient, kyma, v1beta2.KymaKind.Plural())
	if err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			return false, fmt.Errorf("failed to fetch module template CRDs and update Kyma annotations: %w", err)
		}
	}

	moduleTemplateCrdUpdated, err := fetchCrdsAndUpdateKymaAnnotations(ctx, controlPlaneClient,
		runtimeClient, kyma, v1beta2.ModuleTemplateKind.Plural())
	if err != nil {
		err = client.IgnoreNotFound(err)
		if err != nil {
			return false, fmt.Errorf("failed to fetch kyma CRDs and update Kyma annotations: %w", err)
		}
	}

	return kymaCrdUpdated || moduleTemplateCrdUpdated, nil
}

func fetchCrdsAndUpdateKymaAnnotations(ctx context.Context, controlPlaneClient Client,
	runtimeClient Client, kyma *v1beta2.Kyma, plural string,
) (bool, error) {
	kcpCrd, skrCrd, err := fetchCrds(ctx, controlPlaneClient, runtimeClient, plural)
	if err != nil {
		return false, err
	}
	crdUpdated, err := updateRemoteCRD(ctx, kyma, runtimeClient, skrCrd, kcpCrd)
	if err != nil {
		return false, err
	}
	if crdUpdated {
		err = runtimeClient.Get(
			ctx, client.ObjectKey{
				Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
			}, skrCrd,
		)
		if err != nil {
			return false, fmt.Errorf("failed to get SKR CRD: %w", err)
		}
		updateKymaAnnotations(kyma, kcpCrd, KCP)
		updateKymaAnnotations(kyma, skrCrd, SKR)
	}

	return crdUpdated, nil
}

func fetchCrds(ctx context.Context, controlPlaneClient Client, runtimeClient Client, plural string) (
	*v1extensions.CustomResourceDefinition, *v1extensions.CustomResourceDefinition, error,
) {
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}

	kcpCrdName := fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group)

	crd, ok := cache.GetCachedCRD(kcpCrdName)
	if !ok {
		crd = v1extensions.CustomResourceDefinition{}
		err := controlPlaneClient.Get(
			ctx, client.ObjectKey{Name: kcpCrdName}, &crd,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch CRDs from kcp: %w", err)
		}
		cache.SetCRDInCache(kcpCrdName, crd)
	}

	err := runtimeClient.Get(
		ctx, client.ObjectKey{
			Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
		}, crdFromRuntime,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch CRDs from runtime: %w", err)
	}

	return &crd, crdFromRuntime, nil
}

func ContainsLatestVersion(crdFromRuntime *v1extensions.CustomResourceDefinition, latestVersion string) bool {
	for _, version := range crdFromRuntime.Spec.Versions {
		if latestVersion == version.Name {
			return true
		}
	}
	return false
}

func CRDNotFoundErr(err error) bool {
	groupErr := &discovery.ErrGroupDiscoveryFailed{}
	if !errors.As(err, &groupErr) {
		return false
	}
	for _, err := range groupErr.Groups {
		if cannotFoundResource(err) {
			return true
		}
	}
	return false
}

func cannotFoundResource(err error) bool {
	var apiStatusErr k8serrors.APIStatus
	if ok := errors.As(err, &apiStatusErr); ok && apiStatusErr.Status().Details != nil {
		for _, cause := range apiStatusErr.Status().Details.Causes {
			if cause.Type == metav1.CauseTypeUnexpectedServerResponse &&
				strings.Contains(cause.Message, "not found") {
				return true
			}
		}
	}
	return false
}
