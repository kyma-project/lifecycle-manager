package secret

import (
	"context"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Repository struct {
	kcpClient client.Client
	namespace string
}

func NewRepository(kcpClient client.Client, namespace string) *Repository {
	return &Repository{
		kcpClient,
		namespace,
	}
}

func (r *Repository) Get(ctx context.Context, name string) (*apicorev1.Secret, error) {
	secret := &apicorev1.Secret{}
	key := client.ObjectKey{Name: name, Namespace: r.namespace}
	err := r.kcpClient.Get(ctx, key, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s-%s: %w", name, r.namespace, err)
	}

	return secret, nil
}

func (r *Repository) Exists(ctx context.Context, kymaName string) (bool, error) {
	secret, err := r.Get(ctx, kymaName)
	return secret != nil && secret.GetName() != "", err
}

func (r *Repository) List(ctx context.Context, labelSelector k8slabels.Selector) (*apicorev1.SecretList, error) {
	secretList := &apicorev1.SecretList{}
	err := r.kcpClient.List(ctx, secretList, &client.ListOptions{
		LabelSelector: labelSelector, Namespace: r.namespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret by label (%s): %w", labelSelector, err)
	}
	return secretList, nil
}

func (r *Repository) Delete(ctx context.Context, name string) error {
	secret := &apicorev1.Secret{}
	secret.SetName(name)
	secret.SetNamespace(r.namespace)

	if err := r.kcpClient.Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to delete secret %s-%s: %w", name, r.namespace, err)
	}

	return nil
}
