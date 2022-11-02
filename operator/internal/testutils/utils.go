package testutils

import (
	"context"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
)

func IsChartRemoved(ctx context.Context, k8sClient client.Client) func() bool {
	return func() bool {
		err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.WebhookConfigNameTpl),
		}, &admissionv1.ValidatingWebhookConfiguration{})
		if !apierrors.IsNotFound(err) {
			return false
		}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.SecretNameTpl),
		}, &corev1.Secret{})
		if !apierrors.IsNotFound(err) {
			return false
		}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.ServiceAndDeploymentNameTpl),
		}, &appsv1.Deployment{})
		if !apierrors.IsNotFound(err) {
			return false
		}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.ServiceAndDeploymentNameTpl),
		}, &corev1.Service{})
		if !apierrors.IsNotFound(err) {
			return false
		}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ResolveSKRChartResourceName(deploy.ServiceAccountNameTpl),
		}, &corev1.ServiceAccount{})
		if !apierrors.IsNotFound(err) {
			return false
		}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ClusterRoleName,
		}, &rbacv1.ClusterRole{})
		if !apierrors.IsNotFound(err) {
			return false
		}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Namespace: metav1.NamespaceDefault,
			Name:      deploy.ClusterRoleBindingName,
		}, &rbacv1.ClusterRoleBinding{})
		return apierrors.IsNotFound(err)
	}
}

func CreateLoadBalancer(ctx context.Context, controlPlaneClient client.Client) error {
	loadBalancerService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.IngressServiceName,
			Namespace: deploy.IstioSytemNs,
			Labels: map[string]string{
				"app": deploy.IngressServiceName,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Name:       "http2",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	if err := controlPlaneClient.Create(ctx, loadBalancerService); err != nil {
		return err
	}
	loadBalancerService.Status = corev1.ServiceStatus{
		LoadBalancer: corev1.LoadBalancerStatus{
			Ingress: []corev1.LoadBalancerIngress{
				{
					IP: "10.10.10.167",
				},
			},
		},
	}
	if err := controlPlaneClient.Status().Update(ctx, loadBalancerService); err != nil {
		return err
	}

	return controlPlaneClient.Get(ctx, client.ObjectKey{
		Name:      deploy.IngressServiceName,
		Namespace: deploy.IstioSytemNs,
	}, loadBalancerService)
}

func CreateKymaCR(kymaName string) *v1alpha1.Kyma {
	return &v1alpha1.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.KymaKind),
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: v1alpha1.KymaSpec{
			Channel: v1alpha1.ChannelStable,
			Modules: []v1alpha1.Module{
				{
					Name: "sample-skr-module",
				},
				{
					Name: "sample-kcp-module",
				},
			},
			Sync: v1alpha1.Sync{
				Enabled:  false,
				Strategy: v1alpha1.SyncStrategyLocalClient,
			},
		},
	}
}
