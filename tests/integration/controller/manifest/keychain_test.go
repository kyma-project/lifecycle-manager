package manifest_test

import (
	"os"
	"path/filepath"

	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	"github.com/kyma-project/lifecycle-manager/tests/integration"
	"github.com/kyma-project/lifecycle-manager/tests/integration/controller/manifest/manifesttest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe(
	"test authnKeyChain", func() {
		It(
			"should fetch authnKeyChain from secret correctly", FlakeAttempts(5), func() {
				By("install secret")
				const CredSecretLabelValue = "test-operator"
				Eventually(installCredSecret(CredSecretLabelValue), standardTimeout, standardInterval).Should(Succeed())
				const repo = "test.registry.io"
				imageSpecWithCredSelect := CreateOCIImageSpecWithCredSelect("imageName", repo,
					"digest", CredSecretLabelValue)
				keychain, err := ocmextensions.GetAuthnKeychain(manifesttest.Ctx, imageSpecWithCredSelect.CredSecretSelector, manifesttest.K8sClient)
				Expect(err).ToNot(HaveOccurred())
				dig := &TestRegistry{target: repo, registry: repo}
				authenticator, err := keychain.Resolve(dig)
				Expect(err).ToNot(HaveOccurred())
				authConfig, err := authenticator.Authorization()
				Expect(err).ToNot(HaveOccurred())
				Expect(authConfig.Username).To(Equal("test_user"))
				Expect(authConfig.Password).To(Equal("test_pass"))
			},
		)
	},
)

func CreateOCIImageSpecWithCredSelect(name, repo, digest, secretLabelValue string) v1beta2.ImageSpec {
	imageSpec := v1beta2.ImageSpec{
		Name:               name,
		Repo:               repo,
		Type:               "oci-ref",
		Ref:                digest,
		CredSecretSelector: manifesttest.CredSecretLabelSelector(secretLabelValue),
	}
	return imageSpec
}

type TestRegistry struct {
	target   string
	registry string
}

func (d TestRegistry) String() string {
	return d.target
}

func (d TestRegistry) RegistryStr() string {
	return d.registry
}

func installCredSecret(secretLabelValue string) func() error {
	return func() error {
		secret := &apicorev1.Secret{}
		secretFile, err := os.ReadFile(filepath.Join(integration.GetProjectRoot(), "pkg", "test_samples", "auth_secret.yaml"))
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(secretFile, secret)
		Expect(err).ToNot(HaveOccurred())
		secret.Labels[manifesttest.CredSecretLabelKeyForTest] = secretLabelValue
		err = manifesttest.K8sClient.Create(manifesttest.Ctx, secret)
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		Expect(err).ToNot(HaveOccurred())
		return manifesttest.K8sClient.Get(manifesttest.Ctx, client.ObjectKeyFromObject(secret), &apicorev1.Secret{})
	}
}
