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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/pkg/certmanager"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"

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

	// Fetch new/updated Secret
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, secret)
	if err != nil {
		return ctrl.Result{}, err
	}
	kymaName := strings.TrimSuffix(secret.Name, certmanager.CertificateSuffix)

	// Fetch corresponding KymaCR
	kyma := &v1alpha1.Kyma{}
	err = r.Get(ctx, types.NamespacedName{
		Namespace: req.Namespace,
		Name:      kymaName,
	}, kyma)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Set secret namespace to Kyma-Sync Namespace
	remoteSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secret.Name,
			Namespace:   kyma.Spec.Sync.Namespace,
			Labels:      secret.Labels,
			Annotations: secret.Annotations,
		},
		Data:       secret.Data,
		StringData: secret.StringData,
	}

	// Create/Update secret on remote client
	skrClient, err := remote.NewRemoteClient(ctx, r.Client, client.ObjectKeyFromObject(kyma),
		kyma.Spec.Sync.Strategy, r.RemoteClientCache)
	err = skrClient.Get(ctx, types.NamespacedName{
		Namespace: remoteSecret.Namespace,
		Name:      remoteSecret.Name,
	}, &corev1.Secret{})
	if errors.IsNotFound(err) {
		err = skrClient.Get(ctx, types.NamespacedName{Name: remoteSecret.Namespace}, &corev1.Namespace{})
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Target namespace %s doesn't exist, creating it", remoteSecret.Namespace))
			err = skrClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: remoteSecret.Namespace},
			})
			if err != nil {
				return ctrl.Result{}, err
			}
		} else if err != nil {
			return ctrl.Result{}, err
		}
		logger.Info(fmt.Sprintf("Target secret %s doesn't exist, creating it", remoteSecret))
		err = skrClient.Create(ctx, remoteSecret)
		if err != nil {
			return reconcile.Result{}, err
		}
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Target secret already %s exists, updating it now", remoteSecret))
	err = skrClient.Update(ctx, remoteSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}
