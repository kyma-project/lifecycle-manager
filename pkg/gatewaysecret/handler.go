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

var (
	errCouldNotGetLastModifiedAt = errors.New("getting lastModifiedAt time failed")
	errExpectedOneRootCASecret   = errors.New("expected exactly one root CA secret")
)

type Handler struct {
	kcpClient    client.Client
	kcpClientset *kubernetes.Clientset
	log          logr.Logger
}

func NewGatewaySecretHandler(kcpClient client.Client, kcpClientset *kubernetes.Clientset,
	log logr.Logger,
) *Handler {
	return &Handler{
		kcpClient:    kcpClient,
		kcpClientset: kcpClientset,
		log:          log,
	}
}

func (h *Handler) manageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret, err := h.FindGatewaySecret(ctx)

	if util.IsNotFound(err) {
		return h.handleNonExisting(ctx, rootSecret)
	}
	if err != nil {
		return err
	}

	return h.handleExisting(ctx, rootSecret, gwSecret)
}

func (h *Handler) handleNonExisting(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret := NewGatewaySecret(rootSecret)
	return h.Create(ctx, gwSecret)
}

func (h *Handler) handleExisting(ctx context.Context,
	rootSecret *apicorev1.Secret, gwSecret *apicorev1.Secret,
) error {
	caCert, err := h.GetRootCACertificate(ctx)
	if err != nil {
		return err
	}
	if !RequiresUpdate(gwSecret, caCert) {
		return nil
	}
	CopyRootSecretDataIntoGatewaySecret(gwSecret, rootSecret)
	return h.Update(ctx, gwSecret)
}

func CopyRootSecretDataIntoGatewaySecret(gwSecret *apicorev1.Secret, rootSecret *apicorev1.Secret) {
	gwSecret.Data["tls.crt"] = rootSecret.Data["tls.crt"]
	gwSecret.Data["tls.key"] = rootSecret.Data["tls.key"]
	gwSecret.Data["ca.crt"] = rootSecret.Data["ca.crt"]
}

func RequiresUpdate(gwSecret *apicorev1.Secret, caCert certmanagerv1.Certificate) bool {
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

func (h *Handler) FindGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	return GetGatewaySecret(ctx, h.kcpClient)
}

func (h *Handler) Create(ctx context.Context, secret *apicorev1.Secret) error {
	h.updateLastModifiedAt(secret)
	if err := h.kcpClient.Create(ctx, secret); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
	}
	return nil
}

func (h *Handler) Update(ctx context.Context, secret *apicorev1.Secret) error {
	h.updateLastModifiedAt(secret)
	if err := h.kcpClient.Update(ctx, secret); err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
	}
	return nil
}

func (h *Handler) GetRootCACertificate(ctx context.Context) (certmanagerv1.Certificate, error) {
	caCert := certmanagerv1.Certificate{}
	if err := h.kcpClient.Get(ctx,
		client.ObjectKey{Namespace: istioNamespace, Name: kcpCACertName},
		&caCert); err != nil {
		return certmanagerv1.Certificate{}, fmt.Errorf("failed to get CA certificate: %w", err)
	}
	return caCert, nil
}

func (h *Handler) updateLastModifiedAt(secret *apicorev1.Secret) {
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

func (h *Handler) StartRootCertificateWatch() {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	h.handleAlreadyCreatedRootCertificate(ctx)
	h.handleNewRootCertificates(ctx)
}

func (h *Handler) handleAlreadyCreatedRootCertificate(ctx context.Context) {
	rootCASecrets, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).List(ctx, apimetav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(apimetav1.ObjectNameField, kcpRootSecretName).String(),
	})
	if err != nil {
		h.log.Error(err, "unable to list root certificate")
		panic(err)
	}
	if len(rootCASecrets.Items) != 1 {
		h.log.Error(errExpectedOneRootCASecret, errExpectedOneRootCASecret.Error(),
			"found", len(rootCASecrets.Items))
		panic(fmt.Errorf("%w: found %d", errExpectedOneRootCASecret, len(rootCASecrets.Items)))
	}
	rootCASecret := &rootCASecrets.Items[0]
	err = h.manageGatewaySecret(ctx, rootCASecret)
	if err != nil {
		panic(err)
	}
}

func (h *Handler) handleNewRootCertificates(ctx context.Context) {
	secretWatch, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).Watch(ctx, apimetav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(apimetav1.ObjectNameField, kcpRootSecretName).String(),
	})
	if err != nil {
		h.log.Error(err, "unable to start watching root certificate")
		panic(err)
	}

	WatchEvents(ctx, secretWatch.ResultChan(), h.manageGatewaySecret, h.log)
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
