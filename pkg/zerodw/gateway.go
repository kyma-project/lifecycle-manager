package zerodw

import (
	"context"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
	"time"

	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GatewaySecretName = "gateway-secret"
	kcpRootSecretName = "klm-watcher"
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
		// gateway secret does not exist
		return gsh.handleNonExisting(rootSecret)
	}
	if err != nil {
		return err
	}

	// gateway secret exists
	return gsh.handleExisting(rootSecret, gwSecret)
}

func (gsh *GatewaySecretHandler) handleNonExisting(rootSecret *apicorev1.Secret) error {
	// create gateway secret
	gwSecret := gsh.newGatewaySecret(rootSecret)
	err := gsh.create(context.TODO(), gwSecret)
	if err == nil {
		gsh.log.Info("created the gateway secret", "reason", "gateway secret does not exist")
	}
	return err
}

func (gsh *GatewaySecretHandler) handleExisting(rootSecret *apicorev1.Secret, gwSecret *apicorev1.Secret) error {
	doUpdate := true

	gwSecretlastModifiedAtValue, ok := gwSecret.Annotations[LastModifiedAtAnnotation]
	if ok {
		gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretlastModifiedAtValue)
		if err == nil && gwSecretLastModifiedAt.Before(rootSecret.CreationTimestamp.Time) {
			doUpdate = false
		}
	}

	// update gateway secret if creation time of kcp secret is newer than the gateway Secret
	if doUpdate {
		gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
		gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
		gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
		err := gsh.update(context.TODO(), gwSecret)
		if err == nil {
			gsh.log.Info("updated the gateway secret", "reason", "CA-Bundle is more recent than the gateway secret")
		}
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
