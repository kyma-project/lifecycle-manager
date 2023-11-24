//nolint:gochecknoglobals
package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistryv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/onsi/gomega"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	machineryruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
)

var (
	Cancel                   context.CancelFunc
	Ctx                      context.Context
	K8sClient                client.Client
	Server                   *httptest.Server
	ErrManifestStateMisMatch = errors.New("ManifestState mismatch")
	ManifestFilePath         = filepath.Join(integration.GetProjectRoot(), "pkg", "test_samples", "oci", "rendered.yaml")
)

const (
	CredSecretLabelKeyForTest = "operator.kyma-project.io/oci-registry-cred" //nolint:gosec
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

//nolint:ireturn //external dependency used here already returns an interface
func CreateImageSpecLayer() containerregistryv1.Layer {
	layer, err := partial.UncompressedToLayer(mockLayer{filePath: ManifestFilePath})
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	return layer
}

func PushToRemoteOCIRegistry(layerName string) {
	layer := CreateImageSpecLayer()
	digest, err := layer.Digest()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	// Set up a fake registry and write what we pulled to it.
	u, err := url.Parse(Server.URL)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	dst := fmt.Sprintf("%s/%s@%s", u.Host, layerName, digest)
	ref, err := name.NewDigest(dst)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	err = remote.WriteLayer(ref.Context(), layer)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	got, err := remote.Layer(ref)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gotHash, err := got.Digest()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(gotHash).To(gomega.Equal(digest))
}

func CreateOCIImageSpec(name, repo string, enableCredSecretSelector bool) v1beta2.ImageSpec {
	imageSpec := v1beta2.ImageSpec{
		Name: name,
		Repo: repo,
		Type: "oci-ref",
	}
	if enableCredSecretSelector {
		imageSpec.CredSecretSelector = CredSecretLabelSelector("test-secret-label")
	}
	layer := CreateImageSpecLayer()
	digest, err := layer.Digest()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	imageSpec.Ref = digest.String()
	return imageSpec
}

func WithInvalidInstallImageSpec(enableResource bool) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		invalidImageSpec := CreateOCIImageSpec("invalid-image-spec", "domain.invalid", false)
		imageSpecByte, err := json.Marshal(invalidImageSpec)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		return InstallManifest(manifest, imageSpecByte, enableResource)
	}
}

func WithValidInstallImageSpec(name string,
	enableResource, enableCredSecretSelector bool,
) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		validImageSpec := CreateOCIImageSpec(name, Server.Listener.Addr().String(), enableCredSecretSelector)
		imageSpecByte, err := json.Marshal(validImageSpec)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		return InstallManifest(manifest, imageSpecByte, enableResource)
	}
}

func InstallManifest(manifest *v1beta2.Manifest, installSpecByte []byte, enableResource bool) error {
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
				"apiVersion": "operator.kyma-project.io/v1alpha1",
				"kind":       "Sample",
				"metadata": map[string]interface{}{
					"name":      "sample-cr-" + manifest.GetName(),
					"namespace": apimetav1.NamespaceDefault,
				},
				"namespace": "default",
			},
		}
	}
	err := K8sClient.Create(Ctx, manifest)
	if err != nil {
		return fmt.Errorf("error creating Manifest: %w", err)
	}
	return nil
}

func ExpectManifestStateIn(state shared.State) func(manifestName string) error {
	return func(manifestName string) error {
		status, err := GetManifestStatus(manifestName)
		if err != nil {
			return err
		}
		if state != status.State {
			return fmt.Errorf("status is %v but expected %s: %w", status, state, ErrManifestStateMisMatch)
		}
		return nil
	}
}

func ExpectOCISyncRefAnnotationExists(mustExist bool) func(manifestName string) error {
	return func(manifestName string) error {
		manifest, err := GetManifest(manifestName)
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

func GetManifestStatus(manifestName string) (shared.Status, error) {
	manifest, err := GetManifest(manifestName)
	if err != nil {
		return shared.Status{}, err
	}
	return manifest.Status, nil
}

func GetManifest(manifestName string) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	err := K8sClient.Get(
		Ctx, client.ObjectKey{
			Namespace: apimetav1.NamespaceDefault,
			Name:      manifestName,
		}, manifest,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting Manifest %s: %w", manifestName, err)
	}
	return manifest, nil
}

func DeleteManifestAndVerify(manifest *v1beta2.Manifest) func() error {
	return func() error {
		if err := K8sClient.Delete(Ctx, manifest); err != nil && !util.IsNotFound(err) {
			return fmt.Errorf("error deleting Manifest %s: %w", manifest.Name, err)
		}
		newManifest := v1beta2.Manifest{}
		err := K8sClient.Get(Ctx, client.ObjectKeyFromObject(manifest), &newManifest)
		//nolint:wrapcheck
		return client.IgnoreNotFound(err)
	}
}

func CredSecretLabelSelector(labelValue string) *apimetav1.LabelSelector {
	return &apimetav1.LabelSelector{
		MatchLabels: map[string]string{CredSecretLabelKeyForTest: labelValue},
	}
}
