package zerodw

import (
	"context"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
)

const (
	GatewaySecretName = "istio-gateway-secret"
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

func (gsh *GatewaySecretHandler) ManageGatewaySecret(rootSecret *apicorev1.Secret) error {
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
	err := gsh.create(context.Background(), gwSecret)
	if err == nil {
		gsh.log.Info("created the gateway secret", "reason", "gateway secret does not exist")
	}
	return err
}

func (gsh *GatewaySecretHandler) handleExisting(rootSecret *apicorev1.Secret, gwSecret *apicorev1.Secret) error {
	caCert := certmanagerv1.Certificate{}
	if err := gsh.kcpClient.Get(context.Background(),
		client.ObjectKey{Namespace: istioNamespace, Name: kcpCACertName},
		&caCert); err != nil {
		return fmt.Errorf("failed to get CA certificate: %w", err)
	}

	if gwSecretLastModifiedAtValue, ok := gwSecret.Annotations[LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			if caCert.Status.NotBefore != nil && gwSecretLastModifiedAt.After(caCert.Status.NotBefore.Time) {
				return nil
			}
		}
	}

	gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
	gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
	gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
	err := gsh.update(context.Background(), gwSecret)
	if err == nil {
		gsh.log.Info("updated the gateway secret", "reason", "root ca is more recent than the gateway secret")
	}

	return nil
}

func (gsh *GatewaySecretHandler) findGatewaySecret() (*apicorev1.Secret, error) {
	return gsh.findSecret(context.Background(), client.ObjectKey{
		Name:      GatewaySecretName,
		Namespace: istioNamespace,
	})
}

func (gsh *GatewaySecretHandler) findKcpRootSecret() (*apicorev1.Secret, error) {
	return gsh.findSecret(context.Background(), client.ObjectKey{
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
