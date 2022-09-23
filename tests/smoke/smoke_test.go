package smoke

import (
	"context"
	"flag"
	"github.com/kyma-project/lifecycle-manager/tests/smoke/internal"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"os"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"testing"
	"time"
)

var (
	TestEnv     env.Environment
	ClusterName string
)

var provisionType *string

func init() {
	provisionType = flag.String("provision-type", "kyma-cli",
		"Defines the Provisioning Type")
}

func TestMain(m *testing.M) {
	log.Println("setting up test environment from flags")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}

	flag.Parse()

	log.Println("creating test environment")
	TestEnv = env.NewWithConfig(cfg)

	switch *provisionType {
	case "kyma-cli":
		setupKymaProvisioning("kyma", TestEnv)
	case "kind":
		setupKindProvisioning("kind", TestEnv)
	}

	os.Exit(TestEnv.Run(m))
}

func setupKindProvisioning(cluster string, testEnv env.Environment) {
	log.Println("registering setup hooks")
	testEnv.Setup(
		envfuncs.CreateKindCluster(cluster),
		internal.InstallWithKustomize(
			// you could use "github.com/kyma-project/lifecycle-manager//operator/config/default"
			"../../operator/config/default",
		),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			labels := cfg.Labels()
			if labels == nil {
				labels = make(map[string]string)
			}
			labels["test-type.kyma-project.io"] = "smoke"
			cfg.WithLabels(labels)
			return ctx, nil
		},
	)
	log.Println("registering finish hooks")
	testEnv.Finish(envfuncs.DestroyKindCluster(cluster))
}

func setupKymaProvisioning(cluster string, testEnv env.Environment) {
	log.Println("registering setup hooks")
	testEnv.Setup(
		internal.CreateKymaK3dCluster(cluster),
		internal.InstallWithKustomize(
			// you could use "github.com/kyma-project/lifecycle-manager//operator/config/default"
			"../../operator/config/default",
		),
	)
	log.Println("registering finish hooks")
	testEnv.Finish(internal.DestroyKymaK3dCluster(cluster))
}

func TestControllerManagerSpinsUp(t *testing.T) {
	depFeature := features.New("appsv1/deployment/controller-manager").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := ControllerManagerDeployment("kcp-system", "lifecycle-manager-controller-manager")
			// wait for the deployment to finish becoming available
			err = wait.For(conditions.New(
				client.Resources()).ResourcesFound(&appsv1.DeploymentList{Items: []appsv1.Deployment{dep}}),
			)
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).
		Assess("available", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			client, err := cfg.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			dep := ControllerManagerDeployment("kcp-system", "lifecycle-manager-controller-manager")
			// wait for the deployment to finish becoming available
			err = wait.For(conditions.New(client.Resources()).DeploymentConditionMatch(dep.DeepCopy(),
				appsv1.DeploymentAvailable, corev1.ConditionTrue),
				wait.WithTimeout(time.Minute*3))

			pods := corev1.PodList{}
			_ = client.Resources("kcp-system").List(ctx, &pods)
			for _, pod := range pods.Items {
				if marshal, err := yaml.Marshal(&pod.Status); err == nil {
					t.Logf("Pod Status For %s/%s\n%s", pod.Namespace, pod.Name, marshal)
				}
			}

			logDeployStatus(t, ctx, client, dep)

			if err != nil {
				t.Fatal(err)
			}

			return ctx
		}).Feature()

	TestEnv.Test(t, depFeature)
}

func logDeployStatus(t *testing.T, ctx context.Context, client klient.Client, dep appsv1.Deployment) {
	errCheckCtx, cancelErrCheck := context.WithTimeout(ctx, 5*time.Second)
	defer cancelErrCheck()
	if err := client.Resources().Get(errCheckCtx, dep.Name, dep.Namespace, &dep); err != nil {
		t.Error(err)
	}
	if marshal, err := yaml.Marshal(&dep.Status); err == nil {
		t.Logf("%s", marshal)
	}
}

func ControllerManagerDeployment(namespace string, name string) appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace,
			Labels: map[string]string{"app.kubernetes.io/component": "lifecycle-manager.kyma-project.io"}},
	}
}
