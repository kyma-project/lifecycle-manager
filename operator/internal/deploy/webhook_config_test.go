package deploy_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/internal/deploy"
)

const (
	webhookChartPath = "../charts/skr-webhook"
	memoryLimits     = "500Mi"
	cpuLimits        = "1"
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
		kymaSample = deploy.CreateKymaCR("kyma-sample")
		Expect(kcpClient.Create(ctx, kymaSample)).To(Succeed())
		Expect(kcpClient.Create(ctx, watcherCR)).To(Succeed())
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(kcpClient.Delete(ctx, kymaSample)).To(Succeed())
		Expect(kcpClient.Delete(ctx, watcherCR)).To(Succeed())
	})

	It("webhook manager installs watcher helm chart with correct webhook config", func() {
		updateRequired, err := webhookMgr.InstallWebhookChart(ctx, kymaSample, remoteClientCache, kcpClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(updateRequired).To(BeFalse())
		webhookCfg, err := deploy.GetDeployedWebhook(ctx, kymaSample, remoteClientCache, kcpClient)
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(watcherCR, webhookCfg)).To(BeTrue())
	})

	It("webhook manager removes watcher helm chart from SKR cluster when watcher list is empty", func() {
		err := webhookMgr.RemoveWebhookChart(ctx, kymaSample, remoteClientCache, kcpClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(webhookMgr.IsSkrChartRemoved(ctx, kymaSample, remoteClientCache, kcpClient)).To(BeTrue())
	})
})
