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

// DeployerModuleName is the only module for which
// CreateDeployerModuleImagePullSecretTransform takes effect. The transform
// MUST NOT silently apply to any other module: image-pull-secret data injection is
// reserved for the deployer module per the feature design (issue #3345).
const DeployerModuleName = "deployer"

const secretKind = "Secret"

var (
	// ErrInjectDataFromKCP is the umbrella sentinel for every error returned by
	// CreateDeployerModuleImagePullSecretTransform. The Manifest
	// reconciler keys on it (errors.Is) to mark the manifest as StateError and
	// short-circuit SSA — see renderResourcesForInstall in reconciler.go.
	ErrInjectDataFromKCP = errors.New("inject-data-from-kcp transform failed")

	ErrInjectFromKCPSecretMissingModuleLabel = fmt.Errorf(
		"%w: kcp source secret is missing the required %s label", ErrInjectDataFromKCP, shared.ModuleName)
	ErrInjectFromKCPSecretModuleLabelMismatch = fmt.Errorf(
		"%w: kcp source secret %s label does not match the manifest module", ErrInjectDataFromKCP, shared.ModuleName)
)

// SecretRepository reads Secrets from a fixed namespace (kcp-system).
type SecretRepository interface {
	Get(ctx context.Context, name string) (*apicorev1.Secret, error)
}

// CreateDeployerModuleImagePullSecretTransform returns a ResourceTransform
// that replaces the .data of any Secret resource in the manifest annotated with
// shared.InjectDataFromKCPAnnotation=true with the .data of a same-named Secret
// fetched from the KCP control-plane namespace.
//
// To prevent module A from reading module B's secret data, the KCP secret MUST
// carry the shared.ModuleName label matching the manifest's module-name label.
//
// The transform is a no-op for any module other than the hardcoded
// DeployerModuleName.
func CreateDeployerModuleImagePullSecretTransform(secretRepo SecretRepository) ResourceTransform {
	return func(ctx context.Context, obj Object, resources []*unstructured.Unstructured) error {
		manifest, ok := obj.(*v1beta2.Manifest)
		if !ok {
			return fmt.Errorf("%w, got %T", ErrResourceTransformExpectedManifestType, obj)
		}

		// Hardcoded module-name gate: only the deployer module is allowed to inject
		// secret data from KCP. See DeployerModuleName above.
		moduleName := manifest.GetLabels()[shared.ModuleName]
		if moduleName != DeployerModuleName {
			return nil
		}
		if secretRepo == nil {
			return fmt.Errorf("%w: secret repository is nil", ErrInjectDataFromKCP)
		}

		for _, resource := range resources {
			if resource == nil {
				continue
			}

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
		return fmt.Errorf("%w: failed to fetch kcp secret %q: %w", ErrInjectDataFromKCP, resource.GetName(), err)
	}
	if kcpSecret.Namespace != shared.DefaultControlPlaneNamespace {
		return fmt.Errorf("%w: kcp source secret %q must be in namespace %q (got %q)",
			ErrInjectDataFromKCP, kcpSecret.Name, shared.DefaultControlPlaneNamespace, kcpSecret.Namespace)
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
	//
	// If present, stringData can override data during apply, so remove it to ensure
	// the injected .data is effective.
	unstructured.RemoveNestedField(resource.Object, "stringData")

	encoded := make(map[string]any, len(data))
	for key, value := range data {
		encoded[key] = base64.StdEncoding.EncodeToString(value)
	}
	if err := unstructured.SetNestedMap(resource.Object, encoded, "data"); err != nil {
		return fmt.Errorf("%w: failed to set .data on secret %q: %w", ErrInjectDataFromKCP, resource.GetName(), err)
	}
	return nil
}
