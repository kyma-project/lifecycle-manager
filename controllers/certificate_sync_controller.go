/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"

	"github.com/kyma-project/lifecycle-manager/pkg/certificates"

	"github.com/kyma-project/lifecycle-manager/pkg/remote"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CertificateSyncReconciler reconciles a Secrets object
type CertificateSyncReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	RemoteClientCache *remote.ClientCache
}

//+kubebuilder:rbac:groups=kyma-project.io,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kyma-project.io,resources=secrets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kyma-project.io,resources=secrets/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

func (r *CertificateSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Syncing Certificate Secret")

	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, secret)
	kymaName := strings.TrimSuffix(secret.Name, certificates.CertificateSuffix)
	if err != nil {
		//TODO
	}

	kyma := &v1alpha1.Kyma{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      kymaName,
	}, kyma)
	if err != nil {
		//TODO
	}

	skrClient, err := remote.NewRemoteClient(ctx, r.Client, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, r.RemoteClientCache)

	err = skrClient.Get(ctx, types.NamespacedName{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}, &corev1.Secret{})
	if err != nil && errors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("Target secret %s doesn't exist, creating it", secret))
		err = skrClient.Create(ctx, secret)
		if err != nil {
			return reconcile.Result{}, err
		}
	} else {
		logger.Info(fmt.Sprintf("Target secret already %s exists, updating it now", secret))
		err = skrClient.Update(ctx, secret)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
