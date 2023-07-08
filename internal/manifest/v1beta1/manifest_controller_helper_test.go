package v1beta1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const manifestInstallName = "manifest-test"

type mockLayer struct {
	filePath string
}

func (m mockLayer) Uncompressed() (io.ReadCloser, error) {
	f, err := os.Open(m.filePath)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(f), nil
}

func (m mockLayer) MediaType() (types.MediaType, error) {
	return types.OCIUncompressedLayer, nil
}

func (m mockLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{Algorithm: "fake", Hex: "diff id"}, nil
}

func CreateImageSpecLayer() v1.Layer {
	layer, err := partial.UncompressedToLayer(mockLayer{filePath: "../../../pkg/test_samples/oci/rendered.yaml"})
	Expect(err).ToNot(HaveOccurred())
	return layer
}

func PushToRemoteOCIRegistry(layerName string) {
	layer := CreateImageSpecLayer()
	digest, err := layer.Digest()
	Expect(err).ToNot(HaveOccurred())

	// Set up a fake registry and write what we pulled to it.
	u, err := url.Parse(server.URL)
	Expect(err).NotTo(HaveOccurred())

	dst := fmt.Sprintf("%s/%s@%s", u.Host, layerName, digest)
	ref, err := name.NewDigest(dst)
	Expect(err).ToNot(HaveOccurred())

	err = remote.WriteLayer(ref.Context(), layer)
	Expect(err).ToNot(HaveOccurred())

	got, err := remote.Layer(ref)
	Expect(err).ToNot(HaveOccurred())
	gotHash, err := got.Digest()
	Expect(err).ToNot(HaveOccurred())
	Expect(gotHash).To(Equal(digest))
}

func createOCIImageSpec(name, repo string, enableCredSecretSelector bool) v1beta2.ImageSpec {
	imageSpec := v1beta2.ImageSpec{
		Name: name,
		Repo: repo,
		Type: "oci-ref",
	}
	if enableCredSecretSelector {
		imageSpec.CredSecretSelector = CredSecretLabel()
	}
	layer := CreateImageSpecLayer()
	digest, err := layer.Digest()
	Expect(err).ToNot(HaveOccurred())
	imageSpec.Ref = digest.String()
	return imageSpec
}

func NewTestManifest(prefix string) *v1beta2.Manifest {
	return &v1beta2.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", prefix, rand.Intn(999999)),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				v1beta2.KymaName: string(uuid.NewUUID()),
			},
		},
	}
}

func withInvalidInstallImageSpec(enableResource bool) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		invalidImageSpec := createOCIImageSpec("invalid-image-spec", "domain.invalid", false)
		imageSpecByte, err := json.Marshal(invalidImageSpec)
		Expect(err).ToNot(HaveOccurred())
		return installManifest(manifest, imageSpecByte, enableResource)
	}
}

func withValidInstallImageSpec(name string,
	enableResource, enableCredSecretSelector bool,
) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		validImageSpec := createOCIImageSpec(name, server.Listener.Addr().String(), enableCredSecretSelector)
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		return installManifest(manifest, imageSpecByte, enableResource)
	}
}

func installManifest(manifest *v1beta2.Manifest, installSpecByte []byte, enableResource bool) error {
	if installSpecByte != nil {
		manifest.Spec.Install = v1beta2.InstallInfo{
			Source: runtime.RawExtension{
				Raw: installSpecByte,
			},
			Name: manifestInstallName,
		}
	}
	if enableResource {
		// related CRD definition is in pkg/test_samples/oci/rendered.yaml
		manifest.Spec.Resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "operator.kyma-project.io/v1alpha1",
				"kind":       "Sample",
				"metadata": map[string]interface{}{
					"name":      "sample-crd-from-manifest",
					"namespace": metav1.NamespaceDefault,
				},
				"namespace": "default",
			},
		}
	}
	return k8sClient.Create(ctx, manifest)
}

func expectManifestStateIn(state v2.State) func(manifestName string) error {
	return func(manifestName string) error {
		status, err := getManifestStatus(manifestName)
		if err != nil {
			return err
		}
		if state != status.State {
			return fmt.Errorf("status is %v but expected %s: %w", status, state, ErrManifestStateMisMatch)
		}
		return nil
	}
}

func getManifestStatus(manifestName string) (v2.Status, error) {
	manifest := &v1beta1.Manifest{}
	err := k8sClient.Get(
		ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      manifestName,
		}, manifest,
	)
	if err != nil {
		return v2.Status{}, err
	}
	return v2.Status(manifest.Status), nil
}

func deleteManifestAndVerify(manifest *v1beta2.Manifest) func() error {
	return func() error {
		if err := k8sClient.Delete(ctx, manifest); err != nil && !errors.IsNotFound(err) {
			return err
		}
		newManifest := v1beta2.Manifest{}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(manifest), &newManifest)
		return client.IgnoreNotFound(err)
	}
}

func CredSecretLabel() *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{"operator.kyma-project.io/oci-registry-cred": "test-operator"},
	}
}
