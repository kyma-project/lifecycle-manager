package gatewaysecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
	kcpClient client.Client
}

func NewGatewaySecretHandler(kcpClient client.Client) *GatewaySecretHandler {
	return &GatewaySecretHandler{
		kcpClient: kcpClient,
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
	if !GatewaySecretRequiresUpdate(gwSecret, caCert) {
		return nil
	}
	CopyRootSecretDataIntoGatewaySecret(gwSecret, rootSecret)
	return gsh.Update(ctx, gwSecret)
}

func CopyRootSecretDataIntoGatewaySecret(gwSecret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
	gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
	gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
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

func (gsh *GatewaySecretHandler) FindGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	return GetGatewaySecret(ctx, gsh.kcpClient)
}

func (gsh *GatewaySecretHandler) Create(ctx context.Context, secret *apicorev1.Secret) error {
	gsh.updateLastModifiedAt(secret)
	if err := gsh.kcpClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
	}
	return nil
}

func (gsh *GatewaySecretHandler) Update(ctx context.Context, secret *apicorev1.Secret) error {
	gsh.updateLastModifiedAt(secret)
	if err := gsh.kcpClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
	}
	return nil
}

func (gsh *GatewaySecretHandler) GetRootCACertificate(ctx context.Context) (certmanagerv1.Certificate, error) {
	caCert := certmanagerv1.Certificate{}
	if err := gsh.kcpClient.Get(ctx,
		client.ObjectKey{Namespace: istioNamespace, Name: kcpCACertName},
		&caCert); err != nil {
		return certmanagerv1.Certificate{}, fmt.Errorf("failed to get CA certificate: %w", err)
	}
	return caCert, nil
}

func (gsh *GatewaySecretHandler) updateLastModifiedAt(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}

func NewGatewaySecret(rootSecret *apicorev1.Secret) *apicorev1.Secret {
	gwSecret := &apicorev1.Secret{
		TypeMeta: apimetav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: apicorev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: apimetav1.ObjectMeta{
			Name:      gatewaySecretName,
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

func GetGatewaySecret(ctx context.Context, clnt client.Client) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	if err := clnt.Get(ctx, client.ObjectKey{
		Name:      gatewaySecretName,
		Namespace: istioNamespace,
	}, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", gatewaySecretName, err)
	}
	return secret, nil
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

	WatchEvents(ctx, secretWatch.ResultChan(), gatewaySecretHandler.ManageGatewaySecret, log)
}

func WatchEvents(ctx context.Context, watchEvents <-chan watch.Event,
	manageGatewaySecretFunc func(context.Context, *apicorev1.Secret) error, log logr.Logger,
) {
	for event := range watchEvents {
		rootCASecret, _ := event.Object.(*apicorev1.Secret)

		switch event.Type {
		case watch.Added, watch.Modified:
			err := manageGatewaySecretFunc(ctx, rootCASecret)
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
