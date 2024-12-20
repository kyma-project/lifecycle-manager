package istiogatewaysecret

import (
	"context"
	"errors"
	"fmt"
	"time"

	apicorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlruntime "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/internal/gatewaysecret"
	gatewaysecretclient "github.com/kyma-project/lifecycle-manager/internal/gatewaysecret/client"
	"github.com/kyma-project/lifecycle-manager/internal/pkg/flags"
)

const (
	controllerName    = "istio-controller"
	kcpRootSecretName = "klm-watcher"
)

var errCouldNotGetLastModifiedAt = errors.New("getting lastModifiedAt time failed")

func SetupReconciler(mgr ctrl.Manager, flagVar *flags.FlagVar, options ctrlruntime.Options) error {
	options.MaxConcurrentReconciles = flagVar.MaxConcurrentWatcherReconciles

	clnt := gatewaysecretclient.NewGatewaySecretRotationClient(mgr.GetConfig())
	var parseLastModifiedFunc gatewaysecret.TimeParserFunc = func(secret *apicorev1.Secret) (time.Time, error) {
		if gwSecretLastModifiedAtValue, ok := secret.Annotations[shared.LastModifiedAtAnnotation]; ok {
			if gwSecretLastModifiedAt, err := time.Parse(time.RFC3339, gwSecretLastModifiedAtValue); err == nil {
				return gwSecretLastModifiedAt, nil
			}
		}
		return time.Time{}, errCouldNotGetLastModifiedAt
	}
	handler := gatewaysecret.NewGatewaySecretHandler(clnt, parseLastModifiedFunc)

	var getSecretFunc GetterFunc = func(ctx context.Context, name types.NamespacedName) (*apicorev1.Secret, error) {
		secret := &apicorev1.Secret{}
		err := mgr.GetClient().Get(ctx, name, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get root gateway secret %w", err)
		}

		return secret, nil
	}

	return NewReconciler(getSecretFunc, handler).setupWithManager(mgr, options)
}

func (r *Reconciler) setupWithManager(mgr ctrl.Manager, opts ctrlruntime.Options) error {
	secretPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isRootSecret(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isRootSecret(e.ObjectNew)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		For(&apicorev1.Secret{}).
		Named(controllerName).
		WithOptions(opts).
		WithEventFilter(secretPredicate).
		Complete(r); err != nil {
		return fmt.Errorf("failed to setup manager for istio controller: %w", err)
	}

	return nil
}

func isRootSecret(object client.Object) bool {
	return object.GetNamespace() == shared.IstioNamespace && object.GetName() == kcpRootSecretName
}
