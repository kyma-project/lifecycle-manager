package v1beta1_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/user"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/declarative/v2"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	internalv1beta1 "github.com/kyma-project/lifecycle-manager/internal/manifest/v1beta1"
	. "github.com/onsi/ginkgo/v2"
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

func createOCIImageSpec(name, repo string) v1beta2.ImageSpec {
	imageSpec := v1beta2.ImageSpec{
		Name: name,
		Repo: repo,
		Type: "oci-ref",
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

func deleteHelmChartResources(imageSpec v1beta2.ImageSpec) {
	chartYamlPath := filepath.Join(internalv1beta1.GetFsChartPath(imageSpec), "Chart.yaml")
	Expect(os.RemoveAll(chartYamlPath)).Should(Succeed())
	valuesYamlPath := filepath.Join(internalv1beta1.GetFsChartPath(imageSpec), "values.yaml")
	Expect(os.RemoveAll(valuesYamlPath)).Should(Succeed())
	templatesPath := filepath.Join(internalv1beta1.GetFsChartPath(imageSpec), "templates")
	Expect(os.RemoveAll(templatesPath)).Should(Succeed())
}

func verifyHelmResourcesDeletion(imageSpec v1beta2.ImageSpec) {
	_, err := os.Stat(filepath.Join(internalv1beta1.GetFsChartPath(imageSpec), "Chart.yaml"))
	Expect(os.IsNotExist(err)).To(BeTrue())
	_, err = os.Stat(filepath.Join(internalv1beta1.GetFsChartPath(imageSpec), "values.yaml"))
	Expect(os.IsNotExist(err)).To(BeTrue())
	_, err = os.Stat(filepath.Join(internalv1beta1.GetFsChartPath(imageSpec), "templates"))
	Expect(os.IsNotExist(err)).To(BeTrue())
}

func skipExpect() func() bool {
	return func() bool {
		return true
	}
}

func expectHelmClientCacheExist(expectExist bool) func(cacheKey string) bool {
	return func(cacheKey string) bool {
		clnt := reconciler.ClientCache.GetClientFromCache(cacheKey)
		if expectExist {
			return clnt != nil
		}
		return clnt == nil
	}
}

func withInvalidInstallImageSpec(remote bool) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		invalidImageSpec := createOCIImageSpec("invalid-image-spec", "domain.invalid")
		imageSpecByte, err := json.Marshal(invalidImageSpec)
		Expect(err).ToNot(HaveOccurred())
		return installManifest(manifest, imageSpecByte, remote)
	}
}

func withValidInstallImageSpec(name string, remote bool) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		validImageSpec := createOCIImageSpec(name, server.Listener.Addr().String())
		imageSpecByte, err := json.Marshal(validImageSpec)
		Expect(err).ToNot(HaveOccurred())
		return installManifest(manifest, imageSpecByte, remote)
	}
}

func withValidInstall(installName string, remote bool) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		validInstallImageSpec := createOCIImageSpec(installName, server.Listener.Addr().String())
		installSpecByte, err := json.Marshal(validInstallImageSpec)
		Expect(err).ToNot(HaveOccurred())

		return installManifest(manifest, installSpecByte, remote)
	}
}

func installManifest(manifest *v1beta2.Manifest, installSpecByte []byte, remote bool) error {
	if installSpecByte != nil {
		manifest.Spec.Install = v1beta2.InstallInfo{
			Source: runtime.RawExtension{
				Raw: installSpecByte,
			},
			Name: manifestInstallName,
		}
	}
	// manifest.Spec.CRDs = crdSpec
	if remote {
		manifest.Spec.Remote = true
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
	manifest := &v1beta2.Manifest{}
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
		// reverting permissions for deletion - in case it was changed during tests
		if err := os.Chmod(kustomizeLocalPath, fs.ModePerm); err != nil {
			return err
		}
		if err := k8sClient.Delete(ctx, manifest); err != nil && !errors.IsNotFound(err) {
			return err
		}
		newManifest := v1beta2.Manifest{}
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(manifest), &newManifest)
		return client.IgnoreNotFound(err)
	}
}

func addInstallSpec(specBytes []byte) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		return installManifest(manifest, specBytes, false)
	}
}

func addInstallSpecWithFilePermission(
	specBytes []byte,
	remote bool, fileMode os.FileMode,
) func(manifest *v1beta2.Manifest) error {
	return func(manifest *v1beta2.Manifest) error {
		currentUser, err := user.Current()
		Expect(err).ToNot(HaveOccurred())
		if currentUser.Username == "root" {
			Skip("This test is not suitable for user with root privileges")
		}
		// should not be run as root user
		Expect(currentUser.Username).ToNot(Equal("root"))
		Expect(os.Chmod(kustomizeLocalPath, fileMode)).ToNot(HaveOccurred())
		return installManifest(manifest, specBytes, remote)
	}
}

func expectFileNotExistError() func() bool {
	return func() bool {
		_, err := os.Stat(filepath.Join(kustomizeLocalPath, ManifestDir))
		return os.IsNotExist(err)
	}
}
