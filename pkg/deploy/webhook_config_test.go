package deploy_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	deploy2 "github.com/kyma-project/lifecycle-manager/pkg/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	. "github.com/kyma-project/lifecycle-manager/pkg/testutils"
)

const (
	webhookChartPath = "../../config/skr-webhook"
)

func createIstioNs() error {
	istioNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: deploy2.IstioSytemNs,
		},
	}
	if err := k8sClient.Create(ctx, istioNs); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
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
		kymaSample = CreateKymaCR(kymaName)
		Expect(createIstioNs()).To(Succeed())
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
		Expect(CreateLoadBalancer(ctx, k8sClient)).To(Succeed())
	})

	BeforeEach(func() {
		// clean rendered manifest
		Expect(os.RemoveAll(filepath.Join(webhookChartPath, RenderedManifestDir))).ShouldNot(HaveOccurred())
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
		// clean rendered manifest
		Expect(os.RemoveAll(filepath.Join(webhookChartPath, RenderedManifestDir))).ShouldNot(HaveOccurred())
	})

	It("deploys watcher helm chart with correct webhook config", func() {
		err := deploy2.UpdateWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient, "500Mi", "1")
		Expect(err).ShouldNot(HaveOccurred())
		webhookCfg, err := deploy2.GetDeployedWebhook(ctx, testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy2.IsWebhookConfigured(watcherCR, webhookCfg)).To(BeTrue())
	})

	It("updates webhook config when helm chart is already installed", func() {
		watcherCR.Spec.Field = v1alpha1.SpecField
		err := deploy2.UpdateWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient, "500Mi", "1")
		Expect(err).ShouldNot(HaveOccurred())
		webhookCfg, err := deploy2.GetDeployedWebhook(ctx, testEnv.Config)
		Expect(err).NotTo(HaveOccurred())
		Expect(deploy2.IsWebhookConfigured(watcherCR, webhookCfg)).To(BeTrue())
	})

	It("removes watcher helm chart from SKR cluster when last cr is deleted", func() {
		err := deploy2.RemoveWebhookConfig(ctx, webhookChartPath, watcherCR, testEnv.Config, k8sClient, "500Mi", "1")
		Expect(err).ShouldNot(HaveOccurred())
		Eventually(IsChartRemoved(ctx, k8sClient), Timeout, Interval).Should(BeTrue())
	})
})
