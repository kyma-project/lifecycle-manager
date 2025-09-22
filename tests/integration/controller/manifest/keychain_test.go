package manifest_test

import (
	"os"
	"path/filepath"

	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/keychainprovider"
	"github.com/kyma-project/lifecycle-manager/tests/integration"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	ociSecretName      = "private-oci-registry-cred" //nolint:gosec // test secret
	ociSecretNamespace = "kcp-system"                //nolint:gosec // test secret
	repo               = "test.registry.io"
)

var _ = Describe(
	"test authnKeyChain", func() {
		It(
			"should fetch authnKeyChain from secret correctly", FlakeAttempts(5), func() {
				By("install secret")
				Eventually(installCredSecret(kcpClient), standardTimeout, standardInterval).Should(Succeed())

				keyChainLookup := keychainprovider.NewFromSecretKeyChainProvider(kcpClient,
					types.NamespacedName{Name: ociSecretName, Namespace: ociSecretNamespace})
				keychain, err := keyChainLookup.Get(ctx)
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

func installCredSecret(clnt client.Client) func() error {
	return func() error {
		secret := &apicorev1.Secret{}
		secretFile, err := os.ReadFile(filepath.Join(integration.GetProjectRoot(), "pkg", "test_samples",
			"auth_secret.yaml"))
		Expect(err).ToNot(HaveOccurred())
		err = yaml.Unmarshal(secretFile, secret)
		Expect(err).ToNot(HaveOccurred())
		Expect(secret.Name).To(Equal(ociSecretName))
		Expect(secret.Namespace).To(Equal(ociSecretNamespace))
		err = clnt.Create(ctx, secret)
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		Expect(err).ToNot(HaveOccurred())
		return clnt.Get(ctx, client.ObjectKeyFromObject(secret), &apicorev1.Secret{})
	}
}
