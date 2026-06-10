package v2

import (
	"context"
	"encoding/base64"
	"fmt"
	"slices"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// SecretRepository fetches secrets from the KCP control plane namespace.
type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
}

func CreateRestrictedDefaultModuleImagePullSecretTransform(
	secretRepo SecretRepository, restrictedModules []string,
) ResourceTransform {
	return func(ctx context.Context, obj Object, resources []*unstructured.Unstructured) error {
		manifest, ok := obj.(*v1beta2.Manifest)
		if !ok {
			return fmt.Errorf("%w, got %T", ErrResourceTransformExpectedManifestType, obj)
		}

		moduleName := manifest.GetLabels()[shared.ModuleName]
		if moduleName == "" || !slices.Contains(restrictedModules, moduleName) {
			return nil
		}

		for _, resource := range resources {
			if resource.GetKind() != "Secret" {
				continue
			}
			if resource.GetAnnotations()[shared.ReplaceFromKCPAnnotation] != shared.EnableLabelValue {
				continue
			}

			kcpSecret, err := secretRepo.Get(ctx, resource.GetName())
			if err != nil {
				return fmt.Errorf("failed to get KCP secret %s: %w", resource.GetName(), err)
			}

			data := make(map[string]any, len(kcpSecret.Data))
			for k, v := range kcpSecret.Data {
				data[k] = base64.StdEncoding.EncodeToString(v)
			}
			if err := unstructured.SetNestedMap(resource.Object, data, "data"); err != nil {
				return fmt.Errorf("failed to replace secret data for %s: %w", resource.GetName(), err)
			}
		}
		return nil
	}
}
