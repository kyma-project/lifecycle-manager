package v2

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
)

// RestrictedDefaultModuleDeployerName is the only module for which
// CreateRestrictedDefaultModuleImagePullSecretTransform takes effect. The transform
// MUST NOT silently apply to any other module: image-pull-secret data injection is
// reserved for the deployer module per the feature design (issue #3345).
const RestrictedDefaultModuleDeployerName = "deployer"

const secretKind = "Secret"

var (
	ErrInjectFromKCPSecretMissingModuleLabel = errors.New(
		"kcp source secret is missing the required " + shared.ModuleName + " label")
	ErrInjectFromKCPSecretModuleLabelMismatch = errors.New(
		"kcp source secret " + shared.ModuleName + " label does not match the manifest module")
)

// SecretRepository reads Secrets from a fixed namespace (kcp-system).
// Defined at the consumer side per ADR 001.
type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
}

// CreateRestrictedDefaultModuleImagePullSecretTransform returns a ResourceTransform
// that replaces the .data of any Secret resource in the manifest annotated with
// shared.InjectDataFromKCPAnnotation=true with the .data of a same-named Secret
// fetched from the KCP control-plane namespace.
//
// To prevent module A from reading module B's secret data, the KCP secret MUST
// carry the shared.ModuleName label matching the manifest's module-name label.
//
// The transform is a no-op for any module other than the hardcoded
// RestrictedDefaultModuleDeployerName.
func CreateRestrictedDefaultModuleImagePullSecretTransform(secretRepo SecretRepository) ResourceTransform {
	return func(ctx context.Context, obj Object, resources []*unstructured.Unstructured) error {
		manifest, ok := obj.(*v1beta2.Manifest)
		if !ok {
			return fmt.Errorf("%w, got %T", ErrResourceTransformExpectedManifestType, obj)
		}

		// Hardcoded module-name gate: only the deployer module is allowed to inject
		// secret data from KCP. See RestrictedDefaultModuleDeployerName above.
		moduleName := manifest.GetLabels()[shared.ModuleName]
		if moduleName != RestrictedDefaultModuleDeployerName {
			return nil
		}

		for _, resource := range resources {
			if !isInjectableSecret(resource) {
				continue
			}
			if err := injectDataFromKCP(ctx, secretRepo, moduleName, resource); err != nil {
				return err
			}
		}
		return nil
	}
}

func isInjectableSecret(resource *unstructured.Unstructured) bool {
	if resource.GetKind() != secretKind {
		return false
	}
	return resource.GetAnnotations()[shared.InjectDataFromKCPAnnotation] == shared.EnableLabelValue
}

func injectDataFromKCP(ctx context.Context, secretRepo SecretRepository,
	moduleName string, resource *unstructured.Unstructured,
) error {
	kcpSecret, err := secretRepo.Get(ctx, resource.GetName())
	if err != nil {
		return fmt.Errorf("failed to fetch kcp secret %q: %w", resource.GetName(), err)
	}

	if err := assertSecretBelongsToModule(kcpSecret, moduleName); err != nil {
		return err
	}

	return replaceSecretData(resource, kcpSecret.Data)
}

func assertSecretBelongsToModule(kcpSecret *apicorev1.Secret, moduleName string) error {
	srcModule, present := kcpSecret.Labels[shared.ModuleName]
	switch {
	case !present:
		return fmt.Errorf("%w: secret=%s", ErrInjectFromKCPSecretMissingModuleLabel, kcpSecret.Name)
	case srcModule != moduleName:
		return fmt.Errorf("%w: secret=%s expected=%s actual=%s",
			ErrInjectFromKCPSecretModuleLabelMismatch, kcpSecret.Name, moduleName, srcModule)
	}
	return nil
}

func replaceSecretData(resource *unstructured.Unstructured, data map[string][]byte) error {
	// Secret JSON encoding for .data is map<string, base64-string>; we receive the
	// already-decoded raw bytes from the typed Secret and re-encode here.
	encoded := make(map[string]any, len(data))
	for key, value := range data {
		encoded[key] = base64.StdEncoding.EncodeToString(value)
	}
	if err := unstructured.SetNestedMap(resource.Object, encoded, "data"); err != nil {
		return fmt.Errorf("failed to set .data on secret %q: %w", resource.GetName(), err)
	}
	return nil
}
