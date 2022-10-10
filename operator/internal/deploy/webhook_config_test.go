package deploy_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
)

const (
	webhookChartPath = "../charts/skr-webhook"
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

	return k8sClient.Get(ctx, client.ObjectKey{Name: deploy.IngressServiceName,
		Namespace: deploy.IstioSytemNs}, loadBalancerService)
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
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(ctx, watcherCR, testEnv.Config)).To(BeTrue())
	})

	It("updates webhook config when helm chart is already installed", func() {
		watcherCR.Spec.Field = v1alpha1.SpecField
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(ctx, watcherCR, testEnv.Config)).To(BeTrue())
	})

	It("removes webhook config resource from SKR cluster when last cr is deleted", func() {
		err := deploy.RemoveWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deploy.IsWebhookDeployed(ctx, testEnv.Config)).To(BeFalse())
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
