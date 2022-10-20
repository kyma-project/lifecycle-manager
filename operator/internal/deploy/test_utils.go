package deploy

import (
	"context"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	MemoryLimits = "500Mi"
	CpuLimits    = "1"
)

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

func CreateLoadBalancer(ctx context.Context, k8sClient client.Client) error {
	istioNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: IstioSytemNs,
		},
	}
	if err := k8sClient.Create(ctx, istioNs); err != nil {
		return err
	}
	loadBalancerService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IngressServiceName,
			Namespace: IstioSytemNs,
			Labels: map[string]string{
				"app": IngressServiceName,
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

	if err := k8sClient.Create(ctx, loadBalancerService); err != nil {
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
	if err := k8sClient.Status().Update(ctx, loadBalancerService); err != nil {
		return err
	}

	return k8sClient.Get(ctx, client.ObjectKey{
		Name:      IngressServiceName,
		Namespace: IstioSytemNs,
	}, loadBalancerService)
}
