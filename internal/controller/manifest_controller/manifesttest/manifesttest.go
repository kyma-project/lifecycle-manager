//nolint:gochecknoglobals
package manifesttest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	gmg "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Cancel                   context.CancelFunc
	Ctx                      context.Context
	K8sClient                client.Client
	Server                   *httptest.Server
	ErrManifestStateMisMatch = errors.New("ManifestState mismatch")
	ManifestFilePath         = "../../../pkg/test_samples/oci/rendered.yaml"
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

func (m mockLayer) DiffID() (v1.Hash, error) {
	return v1.Hash{Algorithm: "fake", Hex: "diff id"}, nil
}

func CreateImageSpecLayer() v1.Layer {
	layer, err := partial.UncompressedToLayer(mockLayer{filePath: ManifestFilePath})
	gmg.Expect(err).ToNot(gmg.HaveOccurred())
	return layer
}

func PushToRemoteOCIRegistry(layerName string) {
	layer := CreateImageSpecLayer()
	digest, err := layer.Digest()
	gmg.Expect(err).ToNot(gmg.HaveOccurred())

	// Set up a fake registry and write what we pulled to it.
	u, err := url.Parse(Server.URL)
	gmg.Expect(err).NotTo(gmg.HaveOccurred())

	dst := fmt.Sprintf("%s/%s@%s", u.Host, layerName, digest)
	ref, err := name.NewDigest(dst)
	gmg.Expect(err).ToNot(gmg.HaveOccurred())

	err = remote.WriteLayer(ref.Context(), layer)
	gmg.Expect(err).ToNot(gmg.HaveOccurred())

	got, err := remote.Layer(ref)
	gmg.Expect(err).ToNot(gmg.HaveOccurred())
	gotHash, err := got.Digest()
	gmg.Expect(err).ToNot(gmg.HaveOccurred())
	gmg.Expect(gotHash).To(gmg.Equal(digest))
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
	gmg.Expect(err).ToNot(gmg.HaveOccurred())
	imageSpec.Ref = digest.String()
	return imageSpec
}

func WithInvalidInstallImageSpec(enableResource bool) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		invalidImageSpec := CreateOCIImageSpec("invalid-image-spec", "domain.invalid", false)
		return InstallManifest(manifest, invalidImageSpec, enableResource)
	}
}

func WithValidInstallImageSpec(name string,
	enableResource, enableCredSecretSelector bool,
) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		validImageSpec := CreateOCIImageSpec(name, Server.Listener.Addr().String(), enableCredSecretSelector)
		return InstallManifest(manifest, validImageSpec, enableResource)
	}
}

func InstallManifest(manifest *v1beta2.Manifest, installSpec v1beta2.ImageSpec, enableResource bool) error {
	manifest.Spec.Install = installSpec
	if enableResource {
		// related CRD definition is in pkg/test_samples/oci/rendered.yaml
		manifest.Spec.Resource = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "operator.kyma-project.io/v1alpha1",
				"kind":       "Sample",
				"metadata": map[string]interface{}{
					"name":      "sample-cr-" + manifest.GetName(),
					"namespace": metav1.NamespaceDefault,
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

func ExpectManifestStateIn(state v2.State) func(manifestName string) error {
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

func GetManifestStatus(manifestName string) (v2.Status, error) {
	manifest, err := GetManifest(manifestName)
	if err != nil {
		return v2.Status{}, err
	}
	return v2.Status(manifest.Status), nil
}

func GetManifest(manifestName string) (*v1beta2.Manifest, error) {
	manifest := &v1beta2.Manifest{}
	err := K8sClient.Get(
		Ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
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

func CredSecretLabelSelector(labelValue string) *metav1.LabelSelector {
	return &metav1.LabelSelector{
		MatchLabels: map[string]string{CredSecretLabelKeyForTest: labelValue},
	}
}
