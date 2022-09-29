package deploy_test

import (
	"context"
	"fmt"

	"github.com/kyma-project/runtime-watcher/kcp/internal/deploy"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kyma "github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	watcherv1alpha1 "github.com/kyma-project/runtime-watcher/kcp/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	webhookChartPath = "../../../skr/chart/skr-webhook"
	releaseName      = "watcher"
)

var _ = Describe("deploy watcher", Ordered, func() {
	ctx := context.TODO()
	moduleName := "lifecyle-manager"
	watcherCR := &watcherv1alpha1.Watcher{
		TypeMeta: metav1.TypeMeta{
			Kind:       watcherv1alpha1.WatcherKind,
			APIVersion: watcherv1alpha1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-sample", moduleName),
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				watcherv1alpha1.ManagedBylabel: moduleName,
			},
		},
		Spec: watcherv1alpha1.WatcherSpec{
			ServiceInfo: watcherv1alpha1.Service{
				Port:      8082,
				Name:      fmt.Sprintf("%s-svc", moduleName),
				Namespace: metav1.NamespaceDefault,
			},
			LabelsToWatch: map[string]string{
				fmt.Sprintf("%s-watchable", moduleName): "true",
			},
			Field: watcherv1alpha1.StatusField,
		},
	}
	kymaSample := &kyma.Kyma{}
	BeforeAll(func() {
		kymaName := "kyma-sample"
		kymaSample = createKymaCR(kymaName)
		Expect(k8sClient.Create(ctx, kymaSample)).To(Succeed())
	})

	AfterAll(func() {
		// clean up kyma CR
		Expect(k8sClient.Delete(ctx, kymaSample)).To(Succeed())
	})

	It("deploys watcher helm chart with correct webhook config", func() {
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(ctx, watcherCR, testEnv.Config, releaseName)).To(BeTrue())
	})

	It("updates webhook config when helm chart is already installed", func() {
		watcherCR.Spec.Field = watcherv1alpha1.SpecField
		err := deploy.UpdateWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deploy.IsWebhookConfigured(ctx, watcherCR, testEnv.Config, releaseName)).To(BeTrue())
	})

	It("removes webhook config resource from SKR cluster when last cr is deleted", func() {
		err := deploy.RemoveWebhookConfig(ctx, webhookChartPath, releaseName, watcherCR, testEnv.Config, k8sClient)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(deploy.IsWebhookDeployed(ctx, testEnv.Config, releaseName)).To(BeFalse())
	})
})

func createKymaCR(kymaName string) *kyma.Kyma {
	return &kyma.Kyma{
		TypeMeta: metav1.TypeMeta{
			Kind:       string(kyma.KymaKind),
			APIVersion: kyma.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kymaName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: kyma.KymaSpec{
			Channel: kyma.ChannelStable,
			Modules: []kyma.Module{
				{
					Name: "sample-skr-module",
				},
				{
					Name: "sample-kcp-module",
				},
			},
			Sync: kyma.Sync{
				Enabled:  false,
				Strategy: kyma.SyncStrategyLocalClient,
			},
		},
	}
}
