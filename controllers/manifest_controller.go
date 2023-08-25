package controllers

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal/manifest"

	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

func SetupWithManager(mgr manager.Manager, options controller.Options, checkInterval time.Duration) error {

	codec, err := v1beta2.NewCodec()
	if err != nil {
		return fmt.Errorf("unable to initialize codec: %w", err)
	}

	controllerManagedByManager := ctrl.NewControllerManagedBy(mgr).
		For(&v1beta2.Manifest{}).
		Watches(&v1.Secret{}, handler.Funcs{}).WithOptions(options)

	if controllerManagedByManager.Complete(ManifestReconciler(mgr, codec, checkInterval)) != nil {
		return fmt.Errorf("failed to initialize manifest controller by manager: %w", err)
	}
	return nil
}

func ManifestReconciler(
	mgr manager.Manager, codec *v1beta2.Codec,
	checkInterval time.Duration,
) *declarative.Reconciler {
	kcp := &declarative.ClusterInfo{
		Client: mgr.GetClient(),
		Config: mgr.GetConfig(),
	}
	lookup := &manifest.RemoteClusterLookup{KCP: kcp}
	return declarative.NewFromManager(
		mgr, &v1beta2.Manifest{},
		declarative.WithSpecResolver(
			manifest.NewSpecResolver(kcp, codec),
		),
		declarative.WithCustomReadyCheck(manifest.NewCustomResourceReadyCheck()),
		declarative.WithRemoteTargetCluster(lookup.ConfigResolver),
		manifest.WithClientCacheKey(),
		declarative.WithPostRun{manifest.PostRunCreateCR},
		declarative.WithPreDelete{manifest.PreDeleteDeleteCR},
		declarative.WithPeriodicConsistencyCheck(checkInterval),
		declarative.WithModuleCRDName(manifest.GetModuleCRDName),
	)
}
