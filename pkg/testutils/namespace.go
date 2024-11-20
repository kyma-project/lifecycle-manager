package testutils

import (
	"context"

	apicorev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
)

const (
	RemoteNamespace       = shared.DefaultRemoteNamespace
	ControlPlaneNamespace = "kcp-system"
	IstioNamespace        = "istio-system"
)

func CreateNamespace(ctx context.Context, clnt client.Client, name string) error {
	namespace := &apicorev1.Namespace{
		ObjectMeta: apimetav1.ObjectMeta{
			Name: name,
		},
	}
	err := clnt.Create(ctx, namespace)
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}
