package manifest_controller_test

import (
	"os"

	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	manifestctrltest "github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe(
	"test authnKeyChain", func() {
		It(
			"should fetch authnKeyChain from secret correctly", func() {
				By("install secret")
				const CredSecretLabelValue = "test-operator"
				Eventually(installCredSecret(CredSecretLabelValue), standardTimeout, standardInterval).Should(Succeed())
				const repo = "test.registry.io"
				imageSpecWithCredSelect := CreateOCIImageSpecWithCredSelect("imageName", repo,
					"digest", CredSecretLabelValue)
				keychain, err := ocmextensions.GetAuthnKeychain(manifestctrltest.Ctx, imageSpecWithCredSelect.CredSecretSelector, manifestctrltest.K8sClient)
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
		CredSecretSelector: manifestctrltest.CredSecretLabelSelector(secretLabelValue),
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
		secretFile, err := os.ReadFile("../../../pkg/test_samples/auth_secret.yaml")
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(secretFile, secret)
		Expect(err).ToNot(HaveOccurred())
		secret.Labels[manifestctrltest.CredSecretLabelKeyForTest] = secretLabelValue
		err = manifestctrltest.K8sClient.Create(manifestctrltest.Ctx, secret)
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		Expect(err).ToNot(HaveOccurred())
		return manifestctrltest.K8sClient.Get(manifestctrltest.Ctx, client.ObjectKeyFromObject(secret), &apicorev1.Secret{})
	}
}
