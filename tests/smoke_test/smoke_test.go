//go:build smoke

package smoke_test

import (
	"context"
	"flag"
	"log"
	"os"
	"testing"
	"time"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	v2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const (
	KCP                       = "kcp-system"
	KymaCRNamespace           = "kyma-system"
	kymaName                  = "default-kyma"
	moduleName                = "template-operator"
	moduleDeploymentNamespace = "template-operator-system"
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
func TestDefaultControllerManagerSpinsUp(t *testing.T) {
	deploymentName := "lifecycle-manager-controller-manager"
	moduleDeploymentName := "template-operator-v1-controller-manager"
	manifestName := common.CreateModuleName("kyma-project.io/template-operator", kymaName, moduleName)

	depFeature := features.New("default").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("lifecycle manager deployment available", deploymentAvailable(KCP, deploymentName)).
		Assess("module deployment available", deploymentAvailable(moduleDeploymentNamespace, moduleDeploymentName)).
		Assess("kyma readiness", kymaReady(KymaCRNamespace, kymaName)).
		Assess("manifest synced resources exists", manifestSyncedResources(KymaCRNamespace, manifestName)).
		Assess("module CR exists", resourceExists(KymaCRNamespace, manifestName, "sample-yaml")).
		Feature()

	TestEnv.Test(t, depFeature)
}

//nolint:paralleltest
func TestDefaultControllerManagerModuleUpgrade(t *testing.T) {
	moduleDeploymentName := "template-operator-v2-controller-manager"
	newChannel := "fast"
	manifestName := common.CreateModuleName("kyma-project.io/template-operator", kymaName, moduleName)

	depFeature := features.New("module upgrade").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("switch module to fast channel", switchModuleChannel(KymaCRNamespace, kymaName, newChannel)).
		Assess("module deployment available", deploymentAvailable(moduleDeploymentNamespace, moduleDeploymentName)).
		Assess("manifest synced resources exists", manifestSyncedResources(KymaCRNamespace, manifestName)).
		Assess("kyma readiness", kymaReady(KymaCRNamespace, kymaName)).
		Feature()

	TestEnv.Test(t, depFeature)
}

//nolint:paralleltest
func TestControlPlaneControllerManagerSpinsUp(t *testing.T) {
	deploymentName := "klm-controller-manager"
	depFeature := features.New("control-plane").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("available", deploymentAvailable(KCP, deploymentName)).
		Assess("kyma readiness", kymaReady(KymaCRNamespace, "default-kyma")).
		Feature()

	TestEnv.Test(t, depFeature)
}

func kymaReady(namespace string, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)

		kyma := getKyma(ctx, t, restConfig, name, namespace)
		logObj(ctx, t, restConfig, &kyma)

		return ctx
	}
}

func switchModuleChannel(kymaNamespace, kymaName, newChannel string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)

		kyma := getKyma(ctx, t, restConfig, kymaName, kymaNamespace)
		logObj(ctx, t, restConfig, &kyma)
		if len(kyma.Spec.Modules) < 1 {
			t.Fatal("kyma has no module enabled")
		}
		kyma.Spec.Modules[0].Channel = newChannel
		if err := restConfig.Update(ctx, &kyma); err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}

func getKyma(ctx context.Context,
	t *testing.T,
	resourcesFromConfig *resources.Resources,
	name string,
	namespace string,
) v1beta2.Kyma {
	t.Helper()
	var kyma v1beta2.Kyma
	if err := wait.For(func() (bool, error) {
		if err := resourcesFromConfig.Get(ctx, name, namespace, &kyma); err != nil {
			t.Fatal(err)
		}
		return kyma.Status.State == v1beta2.StateReady, nil
	}); err != nil {
		t.Fatal(err)
	}
	return kyma
}

func manifestSyncedResources(namespace string, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)

		manifest := getManifest(ctx, t, restConfig, name, namespace)
		for _, synced := range manifest.Status.Synced {
			unstruct := synced.ToUnstructured()
			if err := restConfig.Get(ctx, unstruct.GetName(), unstruct.GetNamespace(), unstruct); err != nil {
				t.Logf("got error when check: %s/%s", synced.Namespace, synced.Name)
				t.Fatal(err)
			}
		}
		logObj(ctx, t, restConfig, &manifest)
		return ctx
	}
}

func getRestConfig(t *testing.T, cfg *envconf.Config) *resources.Resources {
	t.Helper()
	resourcesFromConfig, err := resources.New(cfg.Client().RESTConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := v1beta2.AddToScheme(resourcesFromConfig.GetScheme()); err != nil {
		t.Fatal(err)
	}
	return resourcesFromConfig
}

func getManifest(ctx context.Context,
	t *testing.T,
	resourcesFromConfig *resources.Resources,
	name string, namespace string,
) v1beta2.Manifest {
	t.Helper()
	var manifest v1beta2.Manifest
	if err := wait.For(func() (bool, error) {
		if err := resourcesFromConfig.Get(ctx, name, namespace, &manifest); err != nil {
			t.Fatal(err)
		}
		return manifest.Status.State == v2.StateReady, nil
	}); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func resourceExists(namespace, manifestName, moduleCRName string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		manifest := getManifest(ctx, t, restConfig, manifestName, namespace)
		resource := manifest.Spec.Resource
		if moduleCRName != resource.GetName() {
			t.Fatalf("module CR name not match: expect %s, but got %s", moduleCRName, resource.GetName())
		}
		if err := wait.For(func() (bool, error) {
			if err := restConfig.Get(ctx, resource.GetName(), resource.GetNamespace(), resource); err != nil {
				t.Fatal(err)
			}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
		logObj(ctx, t, restConfig, resource)
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

func logObj(ctx context.Context, t *testing.T, r *resources.Resources, obj k8s.Object) {
	t.Helper()
	errCheckCtx, cancelErrCheck := context.WithTimeout(ctx, 5*time.Second)
	defer cancelErrCheck()
	if err := r.Get(errCheckCtx, obj.GetName(), obj.GetNamespace(), obj); err != nil {
		t.Error(err)
	}
	if marshal, err := yaml.Marshal(obj); err == nil {
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
