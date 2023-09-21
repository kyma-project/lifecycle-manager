package manifest_controller_test

import (
	"os"

	hlp "github.com/kyma-project/lifecycle-manager/internal/controller/manifest_controller/manifesttest"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/ocmextensions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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
				keychain, err := ocmextensions.GetAuthnKeychain(hlp.Ctx, imageSpecWithCredSelect.CredSecretSelector, hlp.K8sClient)
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
		CredSecretSelector: hlp.CredSecretLabelSelector(secretLabelValue),
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
		secret := &corev1.Secret{}
		secretFile, err := os.ReadFile("../../pkg/test_samples/auth_secret.yaml")
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(secretFile, secret)
		Expect(err).ToNot(HaveOccurred())
		secret.Labels[hlp.CredSecretLabelKeyForTest] = secretLabelValue
		err = hlp.K8sClient.Create(hlp.Ctx, secret)
		if errors.IsAlreadyExists(err) {
			return nil
		}
		Expect(err).ToNot(HaveOccurred())
		return hlp.K8sClient.Get(hlp.Ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{})
	}
}
