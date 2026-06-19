package v2_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

const (
	testKCPSecretName  = "deployer-image-pull" //nolint:gosec // not a credential, just a Secret resource name
	testDeployerModule = "deployer"
	testOtherModule    = "other-module"
)

// secretRepoStub satisfies declarativev2.SecretRepository for the unit tests.
type secretRepoStub struct {
	secrets  map[string]*apicorev1.Secret
	getError error
}

func (s *secretRepoStub) Get(_ context.Context, name string) (*apicorev1.Secret, error) {
	if s.getError != nil {
		return nil, s.getError
	}
	secret, ok := s.secrets[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return secret, nil
}

func newKCPSecret(name, moduleLabel string, data map[string][]byte) *apicorev1.Secret {
	labels := map[string]string{}
	if moduleLabel != "" {
		labels[shared.ModuleName] = moduleLabel
	}
	return &apicorev1.Secret{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      name,
			Namespace: shared.DefaultControlPlaneNamespace,
			Labels:    labels,
		},
		Data: data,
	}
}

func newManifest(moduleName string) *v1beta2.Manifest {
	manifest := &v1beta2.Manifest{}
	if moduleName != "" {
		manifest.SetLabels(map[string]string{shared.ModuleName: moduleName})
	}
	return manifest
}

func newSecretResource(name string, annotations map[string]string, data map[string]any) *unstructured.Unstructured {
	obj := map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name": name,
		},
	}
	if data != nil {
		obj["data"] = data
	}
	res := &unstructured.Unstructured{Object: obj}
	if len(annotations) > 0 {
		res.SetAnnotations(annotations)
	}
	return res
}

func TestDeployerModuleImagePullSecretTransform_ReplacesData_WhenDeployerAndAnnotated(t *testing.T) {
	t.Parallel()

	srcData := map[string][]byte{".dockerconfigjson": []byte(`{"auths":{"registry":{}}}`)}
	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{
		testKCPSecretName: newKCPSecret(testKCPSecretName, testDeployerModule, srcData),
	}}

	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue},
		map[string]any{".dockerconfigjson": base64.StdEncoding.EncodeToString([]byte("dummy"))})

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{resource})
	require.NoError(t, err)

	gotData, found, err := unstructured.NestedStringMap(resource.Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, base64.StdEncoding.EncodeToString(srcData[".dockerconfigjson"]), gotData[".dockerconfigjson"])
	require.Len(t, gotData, 1)
}

func TestDeployerModuleImagePullSecretTransform_NoOp_WhenManifestIsNotDeployer(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{
		testKCPSecretName: newKCPSecret(testKCPSecretName, testOtherModule,
			map[string][]byte{"k": []byte("v")}),
	}}
	originalData := map[string]any{"k": base64.StdEncoding.EncodeToString([]byte("dummy"))}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue},
		originalData)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testOtherModule), []*unstructured.Unstructured{resource})
	require.NoError(t, err)

	gotData, _, err := unstructured.NestedStringMap(resource.Object, "data")
	require.NoError(t, err)
	require.Equal(t, originalData["k"], gotData["k"])
}

func TestDeployerModuleImagePullSecretTransform_NoOp_WhenManifestHasNoModuleNameLabel(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{}}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(""), []*unstructured.Unstructured{resource})
	require.NoError(t, err)
}

func TestDeployerModuleImagePullSecretTransform_IgnoresSecret_WhenAnnotationMissing(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{}}
	originalData := map[string]any{"k": base64.StdEncoding.EncodeToString([]byte("dummy"))}
	resource := newSecretResource(testKCPSecretName, nil, originalData)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{resource})
	require.NoError(t, err)

	gotData, _, err := unstructured.NestedStringMap(resource.Object, "data")
	require.NoError(t, err)
	require.Equal(t, originalData["k"], gotData["k"])
}

func TestDeployerModuleImagePullSecretTransform_IgnoresNonSecretResource_WithAnnotation(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{}}
	configMap := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":        "decoy",
			"annotations": map[string]any{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue},
		},
	}}

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{configMap})
	require.NoError(t, err)
}

func TestDeployerModuleImagePullSecretTransform_Errors_WhenKCPSecretMissingModuleLabel(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{
		testKCPSecretName: newKCPSecret(testKCPSecretName, "", map[string][]byte{"k": []byte("v")}),
	}}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{resource})
	require.ErrorIs(t, err, declarativev2.ErrInjectFromKCPSecretMissingModuleLabel)
	require.ErrorIs(t, err, declarativev2.ErrInjectDataFromKCP)
}

func TestDeployerModuleImagePullSecretTransform_Errors_WhenKCPSecretLabelMismatch(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{
		testKCPSecretName: newKCPSecret(testKCPSecretName, testOtherModule, map[string][]byte{"k": []byte("v")}),
	}}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{resource})
	require.ErrorIs(t, err, declarativev2.ErrInjectFromKCPSecretModuleLabelMismatch)
	require.ErrorIs(t, err, declarativev2.ErrInjectDataFromKCP)
}

func TestDeployerModuleImagePullSecretTransform_Errors_WhenKCPSecretInWrongNamespace(t *testing.T) {
	t.Parallel()

	wrongNs := newKCPSecret(testKCPSecretName, testDeployerModule, map[string][]byte{"k": []byte("v")})
	wrongNs.Namespace = "default"
	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{testKCPSecretName: wrongNs}}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{resource})
	require.ErrorIs(t, err, declarativev2.ErrInjectDataFromKCP)
}

func TestDeployerModuleImagePullSecretTransform_PropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("kcp api unavailable")
	repo := &secretRepoStub{getError: repoErr}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{resource})
	require.ErrorIs(t, err, repoErr)
	require.ErrorIs(t, err, declarativev2.ErrInjectDataFromKCP)
}

func TestDeployerModuleImagePullSecretTransform_ReplacesAllAnnotatedSecrets(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{
		"secret-a": newKCPSecret("secret-a", testDeployerModule, map[string][]byte{"a": []byte("aa")}),
		"secret-b": newKCPSecret("secret-b", testDeployerModule, map[string][]byte{"b": []byte("bb")}),
	}}
	resourceA := newSecretResource("secret-a",
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)
	resourceB := newSecretResource("secret-b",
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule),
		[]*unstructured.Unstructured{resourceA, resourceB})
	require.NoError(t, err)

	for _, pair := range []struct {
		resource *unstructured.Unstructured
		key      string
		raw      []byte
	}{
		{resourceA, "a", []byte("aa")},
		{resourceB, "b", []byte("bb")},
	} {
		got, _, err := unstructured.NestedStringMap(pair.resource.Object, "data")
		require.NoError(t, err)
		require.Equal(t, base64.StdEncoding.EncodeToString(pair.raw), got[pair.key])
	}
}

func TestDeployerModuleImagePullSecretTransform_SkipsNilResource(t *testing.T) {
	t.Parallel()

	srcData := map[string][]byte{".dockerconfigjson": []byte(`{"auths":{}}`)}
	repo := &secretRepoStub{secrets: map[string]*apicorev1.Secret{
		testKCPSecretName: newKCPSecret(testKCPSecretName, testDeployerModule, srcData),
	}}
	resource := newSecretResource(testKCPSecretName,
		map[string]string{shared.InjectDataFromKCPAnnotation: shared.EnableLabelValue}, nil)

	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)
	err := transform(t.Context(), newManifest(testDeployerModule), []*unstructured.Unstructured{nil, resource})
	require.NoError(t, err)

	gotData, found, err := unstructured.NestedStringMap(resource.Object, "data")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, base64.StdEncoding.EncodeToString(srcData[".dockerconfigjson"]), gotData[".dockerconfigjson"])
}

func TestDeployerModuleImagePullSecretTransform_Errors_WhenObjectIsNotManifest(t *testing.T) {
	t.Parallel()

	repo := &secretRepoStub{}
	transform := declarativev2.CreateDeployerModuleImagePullSecretTransform(repo)

	err := transform(t.Context(), &nonManifestObject{}, nil)
	require.ErrorIs(t, err, declarativev2.ErrResourceTransformExpectedManifestType)
}

// nonManifestObject implements declarativev2.Object without being a *v1beta2.Manifest,
// to exercise the type assertion guard inside the transform.
type nonManifestObject struct {
	apicorev1.ConfigMap
}

func (n *nonManifestObject) GetStatus() shared.Status  { return shared.Status{} }
func (n *nonManifestObject) SetStatus(_ shared.Status) {}
func (n *nonManifestObject) DeepCopyObject() machineryruntime.Object {
	return &nonManifestObject{ConfigMap: n.ConfigMap}
}
