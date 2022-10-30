package deploy_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
)

const (
	webhookChartPath = "../charts/skr-webhook"
	timeout          = time.Second * 10
	interval         = time.Millisecond * 250
)

func createLoadBalancer() error {
	istioNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploy.IstioSytemNs,
		},
	}
	if err := k8sClient.Create(ctx, istioNs); err != nil {
		return err
	}
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
		Name:      deploy.IngressServiceName,
		Namespace: deploy.IstioSytemNs,
	}, loadBalancerService)
}

var _ = Describe("deploy watcher", Ordered, func() {
	ctx := context.TODO()
	moduleName := "lifecyle-manager"
	watcherCR := &v1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(v1alpha1.WatcherKind),
			APIVersion: v1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sample", moduleName),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				v1alpha1.ManagedBylabel: moduleName,
			},
		},
		Spec: v1alpha1.WatcherSpec{
			ServiceInfo: v1alpha1.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", moduleName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", moduleName): "true",
			},
			Field: v1alpha1.StatusField,
		},
	}
	kymaSample := &v1alpha1.Kyma{}
	BeforeAll(func() {
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
		Expect(createLoadBalancer()).To(Succeed())
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
	})

	It("deploys watcher helm chart with correct webhook config", func() {
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient, "500Mi", "1")
		Expect(err).ShouldNot(HaveOccurred())
		webhookCfg, err := deploy.GetDeployedWebhook(ctx, testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(watcherCR, webhookCfg)).To(BeTrue())
	})

	It("updates webhook config when helm chart is already installed", func() {
		watcherCR.Spec.Field = v1alpha1.SpecField
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient, "500Mi", "1")
		Expect(err).ShouldNot(HaveOccurred())
		webhookCfg, err := deploy.GetDeployedWebhook(ctx, testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(watcherCR, webhookCfg)).To(BeTrue())
	})

	It("removes watcher helm chart from SKR cluster when last cr is deleted", func() {
		err := deploy.RemoveWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient, "500Mi", "1")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(isChartRemoved(ctx, k8sClient), timeout, interval).Should(BeTrue())
	})
})

func createKymaCR(kymaName string) *v1alpha1.Kyma {
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

func isChartRemoved(ctx context.Context, k8sClient client.Client) func(g Gomega) bool {
	return func(g Gomega) bool {
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
