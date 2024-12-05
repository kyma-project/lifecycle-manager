package gatewaysecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

type Handler struct {
	certManagerClient *CertManagerClient
	kcpClientset      *kubernetes.Clientset
	log               logr.Logger
}

func NewGatewaySecretHandler(config *rest.Config,
	log logr.Logger,
) *Handler {
	return &Handler{
		certManagerClient: NewCertManagerClient(config),
		kcpClientset:      kubernetes.NewForConfigOrDie(config),
		log:               log,
	}
}

func (h *Handler) ManageGatewaySecret(ctx context.Context, rootSecret *apicorev1.Secret) error {
	gwSecret, err := h.findGatewaySecret(ctx)

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
	caCert, err := h.certManagerClient.GetRootCACertificate(ctx)
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

func GetValidLastModifiedAt(secret *apicorev1.Secret) (time.Time, error) {
	if gwSecretLastModifiedAtValue, ok := secret.Annotations[LastModifiedAtAnnotation]; ok {
		if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
			return gwSecretLastModifiedAt, nil
		}
	}
	return time.Time{}, errCouldNotGetLastModifiedAt
}

func (h *Handler) findGatewaySecret(ctx context.Context) (*apicorev1.Secret, error) {
	secret, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).Get(ctx, gatewaySecretName,
		apimetav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", gatewaySecretName, err)
	}
	return secret, nil
}

func (h *Handler) Create(ctx context.Context, secret *apicorev1.Secret) error {
	h.updateLastModifiedAt(secret)
	if _, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).Create(ctx, secret,
		apimetav1.CreateOptions{}); err != nil {
		return fmt.Errorf("failed to create secret %s: %w", secret.Name, err)
	}
	return nil
}

func (h *Handler) Update(ctx context.Context, secret *apicorev1.Secret) error {
	h.updateLastModifiedAt(secret)
	if _, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).Update(ctx, secret,
		apimetav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
	}
	return nil
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

func (h *Handler) StartRootCertificateWatch() error {
	ctx, cancel := context.WithCancel(context.Background())

	if err := h.handleAlreadyCreatedRootCertificate(ctx); err != nil {
		cancel()
		return err
	}
	if err := h.handleNewRootCertificates(ctx, cancel); err != nil {
		cancel()
		return err
	}
	return nil
}

func (h *Handler) handleAlreadyCreatedRootCertificate(ctx context.Context) error {
	rootCASecret, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).Get(ctx, kcpRootSecretName,
		apimetav1.GetOptions{})
	if err != nil {
		if util.IsNotFound(err) {
			return nil
		}
		h.log.Error(err, "unable to get root certificate")
		return fmt.Errorf("unable to get root certificate: %w", err)
	}
	return h.manageGatewaySecret(ctx, rootCASecret)
}

func (h *Handler) handleNewRootCertificates(ctx context.Context, cancel context.CancelFunc) error {
	secretWatch, err := h.kcpClientset.CoreV1().Secrets(istioNamespace).Watch(ctx, apimetav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(apimetav1.ObjectNameField, kcpRootSecretName).String(),
	})
	if err != nil {
		h.log.Error(err, "unable to start watching root certificate")
		return fmt.Errorf("unable to start watching root certificate: %w", err)
	}

	go WatchEvents(ctx, cancel, secretWatch.ResultChan(), h.manageGatewaySecret, h.log)
	return nil
}

func WatchEvents(ctx context.Context, cancel context.CancelFunc, watchEvents <-chan watch.Event,
	manageGatewaySecretFunc func(context.Context, *apicorev1.Secret) error, log logr.Logger,
) {
	defer cancel()

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
