package gatewaysecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kyma-project/lifecycle-manager/pkg/util"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LastModifiedAtAnnotation = "lastModifiedAt"
	GatewaySecretName        = "klm-istio-gateway" //nolint:gosec // GatewaySecretName is not a credential
	kcpRootSecretName        = "klm-watcher"
	kcpCACertName            = "klm-watcher-serving"
	istioNamespace           = "istio-system"
)

var errCouldNotGetLastModifiedAt = errors.New("getting lastModifiedAt time failed")

type GatewaySecretHandler struct {
	kcpClient client.Client
}

func NewGatewaySecretHandler(kcpClient client.Client) *GatewaySecretHandler {
	return &GatewaySecretHandler{
		kcpClient: kcpClient,
	}
}

func (gsh *GatewaySecretHandler) ManageGatewaySecret(rootSecret *apicorev1.Secret) error {
	gwSecret, err := gsh.findGatewaySecret()

	if util.IsNotFound(err) {
		return gsh.handleNonExisting(rootSecret)
	}
	if err != nil {
		return err
	}

	return gsh.handleExisting(rootSecret, gwSecret)
}

func (gsh *GatewaySecretHandler) handleNonExisting(rootSecret *apicorev1.Secret) error {
	gwSecret := gsh.newGatewaySecret(rootSecret)
	return gsh.create(context.Background(), gwSecret)
}

func (gsh *GatewaySecretHandler) handleExisting(rootSecret *apicorev1.Secret, gwSecret *apicorev1.Secret) error {
	caCert := certmanagerv1.Certificate{}
	if err := gsh.kcpClient.Get(context.Background(),
		client.ObjectKey{Namespace: istioNamespace, Name: kcpCACertName},
		&caCert); err != nil {
		return fmt.Errorf("failed to get CA certificate: %w", err)
	}

	if gwSecretLastModifiedAt, err := GetLastModifiedAt(gwSecret); err == nil {
		if caCert.Status.NotBefore != nil && gwSecretLastModifiedAt.After(caCert.Status.NotBefore.Time) {
			return nil
		}
	}

	gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
	gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
	gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
	return gsh.update(context.Background(), gwSecret)
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

func (gsh *GatewaySecretHandler) findGatewaySecret() (*apicorev1.Secret, error) {
	return gsh.findSecret(context.Background(), client.ObjectKey{
		Name:      GatewaySecretName,
		Namespace: istioNamespace,
	})
}

func (gsh *GatewaySecretHandler) findSecret(ctx context.Context, objKey client.ObjectKey) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}

	err := gsh.kcpClient.Get(ctx, objKey, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", objKey.Name, err)
	}

	return secret, nil
}

func (gsh *GatewaySecretHandler) create(ctx context.Context, secret *apicorev1.Secret) error {
	gsh.updateLastModifiedAt(secret)
	if err := gsh.kcpClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
	}
	return nil
}

func (gsh *GatewaySecretHandler) update(ctx context.Context, secret *apicorev1.Secret) error {
	gsh.updateLastModifiedAt(secret)
	if err := gsh.kcpClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
	}
	return nil
}

func (gsh *GatewaySecretHandler) updateLastModifiedAt(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}

func GetLastModifiedAt(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretLastModifiedAtValue, ok := secret.Annotations[LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			return gwSecretLastModifiedAt, nil
		}
	}
	return time.Time{}, errCouldNotGetLastModifiedAt
}

func WatchChangesOnRootCertificate(clientset *kubernetes.Clientset, gatewaySecretHandler *GatewaySecretHandler,
	log logr.Logger,
) {
	secretWatch, err := clientset.CoreV1().Secrets(istioNamespace).Watch(context.Background(), apimetav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(apimetav1.ObjectNameField, kcpRootSecretName).String(),
	})
	if err != nil {
		log.Error(err, "unable to start watching root certificate")
		panic(err)
	}

	for event := range secretWatch.ResultChan() {
		rootCASecret, _ := event.Object.(*apicorev1.Secret)

		switch event.Type {
		case watch.Added:
			fallthrough
		case watch.Modified:
			err := gatewaySecretHandler.ManageGatewaySecret(rootCASecret)
			if err != nil {
				log.Error(err, "unable to manage istio gateway secret")
			}
		case watch.Deleted:
			// ignored because it is an invalid state and cert manager should not delete the root secret
			// even if it is deleted, the certificate manager will recreate it, and trigger the watch event
			fallthrough
		case watch.Error:
			fallthrough
		case watch.Bookmark:
			fallthrough
		default:
			continue
		}
	}
}
