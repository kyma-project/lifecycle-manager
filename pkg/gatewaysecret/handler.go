package gatewaysecret

import (
	"context"
	"errors"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	LastModifiedAtAnnotation = "lastModifiedAt"
	gatewaySecretName        = "klm-istio-gateway" //nolint:gosec // gatewaySecretName is not a credential
	kcpRootSecretName        = "klm-watcher"
	kcpCACertName            = "klm-watcher-serving"
	istioNamespace           = "istio-system"
)

var errCouldNotGetLastModifiedAt = errors.New("getting lastModifiedAt time failed")

type GatewaySecretHandler struct {
	SecretManager
}

func NewGatewaySecretHandler(secretManager SecretManager) *GatewaySecretHandler {
	return &GatewaySecretHandler{
		secretManager,
	}
}

func (gsh *GatewaySecretHandler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret, err := gsh.FindGatewaySecret(ctx)

	if util.IsNotFound(err) {
		return gsh.handleNonExisting(ctx, rootSecret)
	}
	if err != nil {
		return err
	}

	return gsh.handleExisting(ctx, rootSecret, gwSecret)
}

func (gsh *GatewaySecretHandler) handleNonExisting(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret := NewGatewaySecret(rootSecret)
	return gsh.Create(ctx, gwSecret)
}

func (gsh *GatewaySecretHandler) handleExisting(ctx context.Context,
	rootSecret *apicorev1.Secret, gwSecret *apicorev1.Secret,
) error {
	caCert, err := gsh.GetRootCACertificate(ctx)
	if err != nil {
		return err
	}

	if copied := CopyRootSecretDataIntoGatewaySecret(gwSecret, caCert, rootSecret); !copied {
		return nil
	}

	return gsh.Update(ctx, gwSecret)
}

func CopyRootSecretDataIntoGatewaySecret(gwSecret *apicorev1.Secret, caCert certmanagerv1.Certificate,
	rootSecret *apicorev1.Secret,
) bool {
	if !GatewaySecretRequiresUpdate(gwSecret, caCert) {
		return false
	}

	gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
	gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
	gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
	return true
}

func GatewaySecretRequiresUpdate(gwSecret *apicorev1.Secret, caCert certmanagerv1.Certificate) bool {
	if gwSecretLastModifiedAt, err := GetValidLastModifiedAt(gwSecret); err == nil {
		if caCert.Status.NotBefore != nil && gwSecretLastModifiedAt.After(caCert.Status.NotBefore.Time) {
			return false
		}
	}
	return true
}

func GetValidLastModifiedAt(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretLastModifiedAtValue, ok := secret.Annotations[LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			return gwSecretLastModifiedAt, nil
		}
	}
	return time.Time{}, errCouldNotGetLastModifiedAt
}

func StartRootCertificateWatch(clientset *kubernetes.Clientset, gatewaySecretHandler *GatewaySecretHandler,
	log logr.Logger,
) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	secretWatch, err := clientset.CoreV1().Secrets(istioNamespace).Watch(ctx, apimetav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(apimetav1.ObjectNameField, kcpRootSecretName).String(),
	})
	if err != nil {
		log.Error(err, "unable to start watching root certificate")
		panic(err)
	}

	WatchEvents(ctx, secretWatch.ResultChan(), gatewaySecretHandler, log)
}

func WatchEvents(ctx context.Context, watchEvents <-chan watch.Event,
	gatewaySecretHandler *GatewaySecretHandler, log logr.Logger,
) {
	for event := range watchEvents {
		rootCASecret, _ := event.Object.(*apicorev1.Secret)

		switch event.Type {
		case watch.Added, watch.Modified:
			err := gatewaySecretHandler.ManageGatewaySecret(ctx, rootCASecret)
			if err != nil {
				log.Error(err, "unable to manage istio gateway secret")
			}
		case watch.Deleted:
			// ignored because it is an invalid state and cert manager should not delete the root secret
			// even if it is deleted, the certificate manager will recreate it, and trigger the watch event
			fallthrough
		case watch.Error, watch.Bookmark:
			fallthrough
		default:
			continue
		}
	}
}
