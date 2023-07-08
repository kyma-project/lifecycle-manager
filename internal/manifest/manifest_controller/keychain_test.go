package manifest_controller_test

import (
	"os"

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
				Eventually(installCredSecret(), standardTimeout, standardInterval).Should(Succeed())
				const repo = "test.registry.io"
				imageSpecWithCredSelect := CreateOCIImageSpecWithCredSelect("imageName", repo, "digest")
				keychain, err := ocmextensions.GetAuthnKeychain(ctx, imageSpecWithCredSelect.CredSecretSelector, k8sClient)
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

func CreateOCIImageSpecWithCredSelect(name, repo, digest string) v1beta2.ImageSpec {
	imageSpec := v1beta2.ImageSpec{
		Name:               name,
		Repo:               repo,
		Type:               "oci-ref",
		Ref:                digest,
		CredSecretSelector: CredSecretLabel(),
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

func installCredSecret() func() error {
	return func() error {
		secret := &corev1.Secret{}
		secretFile, err := os.ReadFile("../../../pkg/test_samples/auth_secret.yaml")
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(secretFile, secret)
		Expect(err).ToNot(HaveOccurred())
		err = k8sClient.Create(ctx, secret)
		if errors.IsAlreadyExists(err) {
			return nil
		}
		Expect(err).ToNot(HaveOccurred())
		return k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), &corev1.Secret{})
	}
}
