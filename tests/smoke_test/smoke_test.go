//go:build smoke
// +build smoke

package smoke_test

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta1"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	ControllerDeploymentName = "lifecycle-manager-controller-manager"
	KCP                      = "kcp-system"
	KymaCRNamespace          = "kyma-system"
)

var TestEnv env.Environment //nolint:gochecknoglobals

func TestMain(m *testing.M) {
	log.Println("setting up test environment from flags")
	cfg, err := envconf.NewFromFlags()
	if err != nil {
		panic(err)
	}

	flag.Parse()
	log.Println("creating test environment")

	cfg = cfg.WithKubeconfigFile(conf.ResolveKubeConfigFile())
	log.Println("using kubeconfig in " + cfg.KubeconfigFile())

	TestEnv = env.NewWithConfig(cfg)

	os.Exit(TestEnv.Run(m))
}

//nolint:paralleltest
func TestControllerManagerSpinsUp(t *testing.T) {
	depFeature := features.New("appsv1/deployment/controller-manager").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("exists", deploymentExists(KCP, ControllerDeploymentName)).
		Assess("available", deploymentAvailable(KCP, ControllerDeploymentName)).
		Assess("kyma readiness", kymaReady(KymaCRNamespace, "default-kyma")).
		Feature()

	TestEnv.Test(t, depFeature)
}

func kymaReady(namespace string, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		resourcesFromConfig, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Fatal(err)
		}
		if err := v1beta1.AddToScheme(resourcesFromConfig.GetScheme()); err != nil {
			t.Fatal(err)
		}

		var kyma v1beta1.Kyma
		if err := wait.For(func() (bool, error) {
			if err := resourcesFromConfig.Get(ctx, name, namespace, &kyma); err != nil {
				t.Fatal(err)
			}
			return kyma.Status.State == v1beta1.StateReady, nil
		}); err != nil {
			t.Fatal(err)
		}
		logKymaStatus(ctx, t, resourcesFromConfig, kyma)

		return ctx
	}
}

func deploymentAvailable(namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		client, err := cfg.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		dep := ControllerManagerDeployment(namespace, name)
		// wait for the deployment to finish becoming available
		err = wait.For(
			conditions.New(client.Resources()).DeploymentConditionMatch(
				dep.DeepCopy(),
				appsv1.DeploymentAvailable, corev1.ConditionTrue,
			),
			wait.WithTimeout(time.Minute*3),
		)
		if err != nil {
			t.Fatal(err)
		}

		pods := corev1.PodList{}
		_ = client.Resources(namespace).List(ctx, &pods, func(options *metav1.ListOptions) {
			sel, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
			if err != nil {
				t.Fatal(err)
			}
			options.LabelSelector = sel.String()
		})

		for _, pod := range pods.Items {
			if marshal, err := yaml.Marshal(&pod.Status); err == nil {
				t.Logf("Pod Status Name %s/%s\n%s", pod.Namespace, pod.Name, marshal)
			}
		}

		logDeployStatus(ctx, t, client, dep)

		if err != nil {
			t.Fatal(err)
		}

		return ctx
	}
}

func deploymentExists(namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		client, err := cfg.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		dep := ControllerManagerDeployment(namespace, name)
		// wait for the deployment to finish becoming available
		err = wait.For(
			conditions.New(
				client.Resources(),
			).ResourcesFound(&appsv1.DeploymentList{Items: []appsv1.Deployment{dep}}),
		)
		if err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}

func logKymaStatus(ctx context.Context, t *testing.T, r *resources.Resources, kyma v1beta1.Kyma) {
	t.Helper()
	errCheckCtx, cancelErrCheck := context.WithTimeout(ctx, 5*time.Second)
	defer cancelErrCheck()
	if err := r.Get(errCheckCtx, kyma.Name, kyma.Namespace, &kyma); err != nil {
		t.Error(err)
	}
	if marshal, err := yaml.Marshal(&kyma.Status); err == nil {
		t.Logf("%s", marshal)
	}
}

func logDeployStatus(ctx context.Context, t *testing.T, client klient.Client, dep appsv1.Deployment) {
	t.Helper()
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
		ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: namespace,
			Labels: map[string]string{"app.kubernetes.io/component": "lifecycle-manager.kyma-project.io"},
		},
	}
}

func NewTestKyma(namespace, name string) *v1beta1.Kyma {
	return &v1beta1.Kyma{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1beta1.GroupVersion.String(),
			Kind:       string(v1beta1.KymaKind),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      envconf.RandomName(name, 8),
			Namespace: namespace,
		},
		Spec: v1beta1.KymaSpec{
			Modules: []v1beta1.Module{},
			Channel: v1beta1.DefaultChannel,
		},
	}
}
