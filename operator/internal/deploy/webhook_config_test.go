package deploy_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
	"github.com/kyma-project/lifecycle-manager/operator/internal/testutils"
)

const (
	webhookChartPath = "../charts/skr-webhook"
	timeout          = time.Second * 10
	interval         = time.Millisecond * 250
)

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
		kymaSample = testutils.CreateKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
		Expect(testutils.CreateLoadBalancer(ctx, k8sClient)).To(Succeed())
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
		Eventually(testutils.IsChartRemoved(ctx, k8sClient), timeout, interval).Should(BeTrue())
	})
})
