package zerodw

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"

	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GatewaySecretName = "gateway-secret"
	kcpRootSecretName = "klm-watcher"
	kcpCACertName     = "klm-watcher-serving"
	istioNamespace    = flags.DefaultIstioNamespace
)

type GatewaySecretHandler struct {
	*secretManager
}

func NewGatewaySecretHandler(kcpClient client.Client, log logr.Logger) *GatewaySecretHandler {
	return &GatewaySecretHandler{
		secretManager: &secretManager{
			kcpClient: kcpClient,
			log:       log,
		},
	}
}

func (gsh *GatewaySecretHandler) ManageGatewaySecret() error {
	rootSecret, err := gsh.findKcpRootSecret()
	if err != nil {
		return fmt.Errorf("failed to find the KCP root secret: %w", err)
	}

	gwSecret, err := gsh.findGatewaySecret()

	if isNotFound(err) {
		return gsh.handleNonExisting(rootSecret)
	}
	if err != nil {
		return err
	}

	return gsh.handleExisting(rootSecret, gwSecret)
}

func (gsh *GatewaySecretHandler) handleNonExisting(rootSecret *apicorev1.Secret) error {
	gwSecret := gsh.newGatewaySecret(rootSecret)
	err := gsh.create(context.TODO(), gwSecret)
	if err == nil {
		gsh.log.Info("created the gateway secret", "reason", "gateway secret does not exist")
	}
	return err
}

func (gsh *GatewaySecretHandler) handleExisting(rootSecret *apicorev1.Secret, gwSecret *apicorev1.Secret) error {
	caCert := certmanagerv1.Certificate{}
	if err := gsh.kcpClient.Get(context.TODO(),
		client.ObjectKey{Namespace: istioNamespace, Name: kcpCACertName},
		&caCert); err != nil {
		return fmt.Errorf("failed to get CA certificate: %w", err)
	}

	if gwSecretLastModifiedAtValue, ok := gwSecret.Annotations[LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			if gwSecretLastModifiedAt.After(caCert.Status.NotBefore.Time) {
				return nil
			}
		}
	}

	gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
	gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
	gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
	err := gsh.update(context.TODO(), gwSecret)
	if err == nil {
		gsh.log.Info("updated the gateway secret", "reason", "root ca is more recent than the gateway secret")
	}

	return nil
}

func (gsh *GatewaySecretHandler) findGatewaySecret() (*apicorev1.Secret, error) {
	return gsh.findSecret(context.TODO(), client.ObjectKey{
		Name:      GatewaySecretName,
		Namespace: istioNamespace,
	})
}

func (gsh *GatewaySecretHandler) findKcpRootSecret() (*apicorev1.Secret, error) {
	return gsh.findSecret(context.TODO(), client.ObjectKey{
		Name:      kcpRootSecretName,
		Namespace: istioNamespace,
	})
}

func (gsh *GatewaySecretHandler) newGatewaySecret(rootSecret *apicorev1.Secret) *apicorev1.Secret {
	gwSecret := &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      GatewaySecretName,
			Namespace: istioNamespace,
		},
		Data: map[string][]byte{
			"tls.crt": rootSecret.Data["tls.crt"],
			"tls.key": rootSecret.Data["tls.key"],
			"ca.crt":  rootSecret.Data["ca.crt"],
		},
	}
	return gwSecret
}

func SetupGatewaySecretHandler(kcpClient client.Client, log logr.Logger, gatewaySecretRefreshInterval time.Duration) {
	gatewaySecretHandler := NewGatewaySecretHandler(kcpClient, log)

	go func() {
		for {
			time.Sleep(with10PercentJitter(gatewaySecretRefreshInterval))

			if err := gatewaySecretHandler.ManageGatewaySecret(); err != nil {
				log.Error(err, "failed to manage gateway secret")
				continue
			}
			log.Info("gateway secret managed successfully")
		}
	}()
}

// with10PercentJitter returns a duration with 10% withJitter.
func with10PercentJitter(d time.Duration) time.Duration {
	factor := 0.1
	return withJitter(d, factor)
}

// withJitter returns a duration with jitter. For jitter = 0.1, the returned duration will be between 90% and 110% of the input duration.
func withJitter(d time.Duration, jitter float64) time.Duration {
	return time.Duration(float64(d) * (1 + jitter*randomSymmetricInterval()))
}

// randomSymmetricInterval is a function that returns a random float64 between -1 and 1.
func randomSymmetricInterval() float64 {
	return rand.Float64()*2 - 1 //nolint:gosec // This is not used for security purposes
}
