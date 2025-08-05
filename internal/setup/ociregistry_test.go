package setup_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/internal/setup"

	. "github.com/onsi/gomega"
)

type mockSecretInterface struct {
	mock.Mock
}

func (m *mockSecretInterface) Get(ctx context.Context, name string, opts apimetav1.GetOptions,
) (*apicorev1.Secret, error) {
	args := m.Called(ctx, name, opts)
	return args.Get(0).(*apicorev1.Secret), args.Error(1) //nolint:forcetypeassert // We know the return type is *apicorev1.Secret
}

func TestNewOCIRegistry(t *testing.T) {
	gomega := NewWithT(t)
	mockSecretIface := new(mockSecretInterface)

	_, err := setup.NewOCIRegistry(nil, "", "")
	gomega.Expect(err).To(MatchError(setup.ErrSecretInterfaceNil))

	_, err = setup.NewOCIRegistry(mockSecretIface, "", "")
	gomega.Expect(err).To(MatchError(setup.ErrHostAndCredSecretEmpty))

	_, err = setup.NewOCIRegistry(mockSecretIface, "host", "secret")
	gomega.Expect(err).To(MatchError(setup.ErrBothHostAndCredSecret))

	ociRegistry, err := setup.NewOCIRegistry(mockSecretIface, "host", "")
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(ociRegistry).NotTo(BeNil())

	ociRegistry, err = setup.NewOCIRegistry(mockSecretIface, "", "secret")
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(ociRegistry).NotTo(BeNil())
}

func TestOCIRegistry_ResolveHost_WithHost(t *testing.T) {
	gomega := NewWithT(t)
	mockSecretIface := new(mockSecretInterface)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretIface, "myhost", "")
	host, err := ociRegistry.ResolveHost(t.Context())
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(host).To(Equal("myhost"))
}

func TestOCIRegistry_ResolveHost_WithCredSecret_Success(t *testing.T) {
	gomega := NewWithT(t)
	mockSecretIface := new(mockSecretInterface)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretIface, "", "mysecret")

	dockerConfig := map[string]interface{}{
		"auths": map[string]interface{}{
			"myregistry.io": map[string]string{"auth": "dXNlcjpwYXNz"},
		},
	}
	jsonDockerConfig, err := json.Marshal(dockerConfig)
	gomega.Expect(err).ToNot(HaveOccurred())
	secret := &apicorev1.Secret{
		Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig},
	}
	mockSecretIface.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil)

	host, err := ociRegistry.ResolveHost(t.Context())
	gomega.Expect(err).ToNot(HaveOccurred())
	gomega.Expect(host).To(Equal("myregistry.io"))
}

func TestOCIRegistry_ResolveHost_WithCredSecret_Errors(t *testing.T) {
	gomega := NewWithT(t)
	mockSecretIface := new(mockSecretInterface)
	ociRegistry, _ := setup.NewOCIRegistry(mockSecretIface, "", "mysecret")

	// SecretInterface.Get returns error
	mockSecretIface.On("Get", mock.Anything, "mysecret", mock.Anything).Return((*apicorev1.Secret)(nil),
		errors.New("get error")).Once()
	host, err := ociRegistry.ResolveHost(t.Context())
	gomega.Expect(err).To(MatchError("get error"))
	gomega.Expect(host).To(BeEmpty())

	// Secret missing .dockerconfigjson
	secret := &apicorev1.Secret{Data: map[string][]byte{"other": {}}}
	mockSecretIface.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	host, err = ociRegistry.ResolveHost(t.Context())
	gomega.Expect(err).To(MatchError(setup.ErrSecretMissingDockerConfig))
	gomega.Expect(host).To(BeEmpty())

	// Invalid JSON
	secret = &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": []byte("notjson")}}
	mockSecretIface.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	host, err = ociRegistry.ResolveHost(t.Context())
	gomega.Expect(err.Error()).To(ContainSubstring("failed to unmarshal docker config json"))
	gomega.Expect(host).To(BeEmpty())

	// No host in auths
	dockerConfig := map[string]interface{}{"auths": map[string]interface{}{}}
	jsonDockerConfig, err := json.Marshal(dockerConfig)
	gomega.Expect(err).ToNot(HaveOccurred())
	secret = &apicorev1.Secret{Data: map[string][]byte{".dockerconfigjson": jsonDockerConfig}}
	mockSecretIface.On("Get", mock.Anything, "mysecret", mock.Anything).Return(secret, nil).Once()
	host, err = ociRegistry.ResolveHost(t.Context())
	gomega.Expect(err).To(MatchError(setup.ErrNoRegistryHostFound))
	gomega.Expect(host).To(BeEmpty())
}
