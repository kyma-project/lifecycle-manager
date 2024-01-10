package testutils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrManifestStateMisMatch = errors.New("ManifestState mismatch")

var (
	ErrManifestResourceIsNil        = errors.New("manifest spec.resource is nil")
	ErrManifestsExist               = errors.New("cluster contains manifest CRs")
	errManifestNotInExpectedState   = errors.New("manifest CR not in expected state")
	errManifestDeletionTimestampSet = errors.New("manifest CR has set DeletionTimeStamp")
)

func NewTestManifest(prefix string) *v1beta2.Manifest {
	return &v1beta2.Manifest{
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", prefix, builder.RandomName()),
			Namespace: apimetav1.NamespaceDefault,
			Labels: map[string]string{
				shared.KymaName: string(uuid.NewUUID()),
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
		module := module
		if module.Name == moduleName {
			manifestKey = *module.Manifest
		}
	}

	return GetManifestWithObjectKey(ctx, clnt, client.ObjectKey{
		Namespace: manifestKey.Namespace,
		Name:      manifestKey.Name,
	})
}

func GetManifestWithObjectKey(ctx context.Context,
	clnt client.Client, obj client.ObjectKey,
) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	if err := clnt.Get(ctx, obj, manifest); err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}
	return manifest, nil
}

func ManifestExistsWithAnnotation(ctx context.Context, clnt client.Client,
	annotationKey, annotationValue string,
) error {
	manifests := v1beta2.ManifestList{}
	if err := clnt.List(ctx, &manifests); err != nil {
		return fmt.Errorf("failed listing manifests: %w", err)
	}

	for _, manifest := range manifests.Items {
		if manifest.Annotations[annotationKey] == annotationValue {
			return nil
		}
	}
	return fmt.Errorf("manifest with annotation `%s: %s` does not exist", annotationKey, annotationValue)
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

func NoManifestExist(ctx context.Context,
	clnt client.Client,
) error {
	manifestList := &v1beta2.ManifestList{}
	if err := clnt.List(ctx, manifestList); err != nil {
		return fmt.Errorf("error listing manifests: %w", err)
	}
	if len(manifestList.Items) == 0 {
		return nil
	}
	return fmt.Errorf("error checking no manifests exist on cluster: %w", ErrManifestsExist)
}

func UpdateManifestState(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
	state shared.State,
) error {
	component, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}
	component.Status.State = state
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

func SetSkipLabelToManifest(
	ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
	ifSkip bool,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return fmt.Errorf("failed to get manifest, %w", err)
	}
	if manifest.Labels == nil {
		manifest.Labels = make(map[string]string)
	}
	manifest.Labels[shared.SkipReconcileLabel] = strconv.FormatBool(ifSkip)
	err = clnt.Update(ctx, manifest)
	if err != nil {
		return fmt.Errorf("failed to update manifest, %w", err)
	}

	return nil
}

func SkipLabelExistsInManifest(ctx context.Context,
	clnt client.Client,
	kymaName,
	kymaNamespace,
	moduleName string,
) bool {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return false
	}

	return manifest.Labels[shared.SkipReconcileLabel] == "true"
}

func CheckManifestIsInState(
	ctx context.Context,
	kymaName, kymaNamespace, moduleName string,
	clnt client.Client,
	expectedState shared.State,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}

	if manifest.Status.State != expectedState {
		return fmt.Errorf("%w: expect %s, but in %s",
			errManifestNotInExpectedState, expectedState, manifest.Status.State)
	}
	return nil
}

func ManifestNoDeletionTimeStampSet(ctx context.Context,
	kymaName, kymaNamespace, moduleName string,
	clnt client.Client,
) error {
	manifest, err := GetManifest(ctx, clnt, kymaName, kymaNamespace, moduleName)
	if err != nil {
		return err
	}

	if !manifest.ObjectMeta.DeletionTimestamp.IsZero() {
		return errManifestDeletionTimestampSet
	}
	return nil
}

const (
	OCIRegistryCredLabelKeyForTest = shared.OperatorGroup + shared.Separator + "oci-registry-cred"
)

type mockLayer struct {
	filePath string
}

func (m mockLayer) Uncompressed() (io.ReadCloser, error) {
	f, err := os.Open(m.filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file %s: %w", m.filePath, err)
	}
	return io.NopCloser(f), nil
}

func (m mockLayer) MediaType() (types.MediaType, error) {
	return types.OCIUncompressedLayer, nil
}

func (m mockLayer) DiffID() (containerregistryv1.Hash, error) {
	return containerregistryv1.Hash{Algorithm: "fake", Hex: "diff id"}, nil
}

func CreateImageSpecLayer(manifestFilePath string) (containerregistryv1.Layer, error) {
	return partial.UncompressedToLayer(mockLayer{filePath: manifestFilePath})
}

func PushToRemoteOCIRegistry(server *httptest.Server, manifestFilePath, layerName string) error {
	layer, err := CreateImageSpecLayer(manifestFilePath)
	if err != nil {
		return err
	}
	digest, err := layer.Digest()
	if err != nil {
		return err
	}

	// Set up a fake registry and write what we pulled to it.
	u, err := url.Parse(server.URL)
	if err != nil {
		return err
	}

	dst := fmt.Sprintf("%s/%s@%s", u.Host, layerName, digest)
	ref, err := name.NewDigest(dst)
	if err != nil {
		return err
	}

	err = remote.WriteLayer(ref.Context(), layer)
	if err != nil {
		return err
	}

	got, err := remote.Layer(ref)
	if err != nil {
		return err
	}
	gotHash, err := got.Digest()
	if err != nil {
		return err
	}
	if gotHash != digest {
		return fmt.Errorf("has not equal to digest")
	}
	return nil
}

func CreateOCIImageSpec(name, repo, manifestFilePath string, enableCredSecretSelector bool) (v1beta2.ImageSpec, error) {
	imageSpec := v1beta2.ImageSpec{
		Name: name,
		Repo: repo,
		Type: "oci-ref",
	}
	if enableCredSecretSelector {
		imageSpec.CredSecretSelector = CredSecretLabelSelector("test-secret-label")
	}
	layer, err := CreateImageSpecLayer(manifestFilePath)
	if err != nil {
		return imageSpec, err
	}
	digest, err := layer.Digest()
	if err != nil {
		return imageSpec, err
	}
	imageSpec.Ref = digest.String()
	return imageSpec, nil
}

func WithInvalidInstallImageSpec(ctx context.Context, clnt client.Client,
	enableResource bool, manifestFilePath string,
) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		invalidImageSpec, err := CreateOCIImageSpec("invalid-image-spec", "domain.invalid", manifestFilePath, false)
		if err != nil {
			return err
		}
		imageSpecByte, err := json.Marshal(invalidImageSpec)
		if err != nil {
			return err
		}
		return InstallManifest(ctx, clnt, manifest, imageSpecByte, enableResource)
	}
}

func WithValidInstallImageSpec(ctx context.Context, clnt client.Client, name, manifestFilePath, serverURL string,
	enableResource, enableCredSecretSelector bool,
) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		validImageSpec, err := CreateOCIImageSpec(name, serverURL, manifestFilePath, enableCredSecretSelector)
		if err != nil {
			return err
		}
		imageSpecByte, err := json.Marshal(validImageSpec)
		if err != nil {
			return err
		}
		return InstallManifest(ctx, clnt, manifest, imageSpecByte, enableResource)
	}
}

func InstallManifest(ctx context.Context, clnt client.Client, manifest *v1beta2.Manifest, installSpecByte []byte,
	enableResource bool,
) error {
	if installSpecByte != nil {
		manifest.Spec.Install = v1beta2.InstallInfo{
			Source: machineryruntime.RawExtension{
				Raw: installSpecByte,
			},
			Name: v1beta2.RawManifestLayerName,
		}
	}
	if enableResource {
		// related CRD definition is in pkg/test_samples/oci/rendered.yaml
		manifest.Spec.Resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": shared.OperatorGroup + shared.Separator + "v1alpha1",
				"kind":       "Sample",
				"metadata": map[string]interface{}{
					"name":      "sample-cr-" + manifest.GetName(),
					"namespace": apimetav1.NamespaceDefault,
				},
				"namespace": "default",
			},
		}
	}
	err := clnt.Create(ctx, manifest)
	if err != nil {
		return fmt.Errorf("error creating Manifest: %w", err)
	}
	return nil
}

func ExpectManifestStateIn(ctx context.Context, clnt client.Client,
	state shared.State,
) func(manifestName string) error {
	return func(manifestName string) error {
		status, err := GetManifestStatus(ctx, clnt, manifestName)
		if err != nil {
			return err
		}
		if state != status.State {
			return fmt.Errorf("status is %v but expected %s: %w", status, state, ErrManifestStateMisMatch)
		}
		return nil
	}
}

func ExpectOCISyncRefAnnotationExists(ctx context.Context, clnt client.Client,
	mustExist bool,
) func(manifestName string) error {
	return func(manifestName string) error {
		manifest, err := GetManifestWithName(ctx, clnt, manifestName)
		if err != nil {
			return err
		}

		annValue := manifest.Annotations["sync-oci-ref"]
		mustNotExist := !mustExist

		if mustExist && annValue == "" {
			return fmt.Errorf("expected \"sync-oci-ref\" annotation does not exist for manifest %s: %w",
				manifestName, ErrManifestStateMisMatch)
		}
		if mustNotExist && annValue != "" {
			return fmt.Errorf("expected \"sync-oci-ref\" annotation to be empty - but it's not - for manifest %s: %w",
				manifestName, ErrManifestStateMisMatch)
		}

		return nil
	}
}

func GetManifestStatus(ctx context.Context, clnt client.Client, manifestName string) (shared.Status, error) {
	manifest, err := GetManifestWithName(ctx, clnt, manifestName)
	if err != nil {
		return shared.Status{}, err
	}
	return manifest.Status, nil
}

func GetManifestWithName(ctx context.Context, clnt client.Client, manifestName string) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	err := clnt.Get(
		ctx, client.ObjectKey{
			Namespace: apimetav1.NamespaceDefault,
			Name:      manifestName,
		}, manifest,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting Manifest %s: %w", manifestName, err)
	}
	return manifest, nil
}

func DeleteManifestAndVerify(ctx context.Context, clnt client.Client, manifest *v1beta2.Manifest) func() error {
	return func() error {
		if err := clnt.Delete(ctx, manifest); err != nil && !util.IsNotFound(err) {
			return fmt.Errorf("error deleting Manifest %s: %w", manifest.Name, err)
		}
		newManifest := v1beta2.Manifest{}
		err := clnt.Get(ctx, client.ObjectKeyFromObject(manifest), &newManifest)
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to fetch manifest %w", err)
		}
		return nil
	}
}

func CredSecretLabelSelector(labelValue string) *apimetav1.LabelSelector {
	return &apimetav1.LabelSelector{
		MatchLabels: map[string]string{OCIRegistryCredLabelKeyForTest: labelValue},
	}
}
