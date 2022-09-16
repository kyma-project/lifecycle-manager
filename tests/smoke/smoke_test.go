package smoke

import (
	"context"
	"github.com/kyma-project/lifecycle-manager/tests/smoke/internal"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"testing"
	"time"
)

var (
	TestEnv     env.Environment
	ClusterName string
)

func TestMain(m *testing.M) {
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}

	TestEnv = env.NewWithConfig(cfg)
	ClusterName = "kyma"

	TestEnv.Setup(
		internal.CreateKymaK3dCluster(ClusterName),
		internal.InstallWithKustomize(
			// you could use "github.com/kyma-project/lifecycle-manager//operator/config/default"
			"../../operator/config/default",
		),
	)

	TestEnv.Finish(
		internal.DestroyKymaK3dCluster(ClusterName),
	)

	os.Exit(TestEnv.Run(m))
}

func TestControllerManagerSpinsUp(t *testing.T) {
	depFeature := features.New("appsv1/deployment/controller-manager").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := ControllerManagerDeployment("kcp-system", "lifecycle-manager-controller-manager")
			// wait for the deployment to finish becoming available
			err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(dep,
				appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*1))
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()

	TestEnv.Test(t, depFeature)
}

func ControllerManagerDeployment(namespace string, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace,
			Labels: map[string]string{"app.kubernetes.io/component": "lifecycle-manager.kyma-project.io"}},
	}
}
