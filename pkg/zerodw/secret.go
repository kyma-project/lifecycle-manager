package zerodw

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	apicorev1 "k8s.io/api/core/v1"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	LastModifiedAtAnnotation = "lastModifiedAt"
)

type secretManager struct {
	log       logr.Logger
	kcpClient client.Client
}

func (sm *secretManager) findSecret(ctx context.Context, objKey client.ObjectKey) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}

	err := sm.kcpClient.Get(ctx, objKey, secret)

	if err != nil {
		return nil, err
	}

	return secret, nil
}

// isNotFound returns true if the error is a NotFound error.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return client.IgnoreNotFound(err) == nil
}

func (sm *secretManager) create(ctx context.Context, secret *apicorev1.Secret) error {
	sm.updateLastModifiedAt(secret)
	return sm.kcpClient.Create(ctx, secret)
}

func (sm *secretManager) update(ctx context.Context, secret *apicorev1.Secret) error {
	sm.updateLastModifiedAt(secret)
	return sm.kcpClient.Update(ctx, secret)
}

func (sm *secretManager) updateLastModifiedAt(secret *apicorev1.Secret) {
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[LastModifiedAtAnnotation] = apimetav1.Now().Format(time.RFC3339)
}
