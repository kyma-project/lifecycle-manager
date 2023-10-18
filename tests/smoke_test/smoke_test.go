//go:build smoke

//nolint:gochecknoglobals,paralleltest
package smoke_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	apiapps "k8s.io/api/apps/v1"
	apicore "k8s.io/api/core/v1"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimachinerymeta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient"
	"sigs.k8s.io/e2e-framework/klient/conf"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	declarative "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/pkg/testutils/builder"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

const (
	KCPNS            = "kcp-system"
	KymaCRNS         = "kyma-system"
	kymaName         = "default-kyma"
	moduleName       = "template-operator"
	moduleCRName     = "sample-yaml"
	moduleOperatorNS = "template-operator-system"
	moduleCRKind     = "Sample"
	moduleCRVersion  = "v1alpha1"
)

var (
	ErrNotDeleted = errors.New("resource not deleted")
	TestEnv       env.Environment
)

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

func TestDefaultControllerManagerSpinsUp(t *testing.T) {
	deploymentName := "lifecycle-manager-controller-manager"
	moduleOperatorName := "template-operator-v1-controller-manager"
	manifestName := common.CreateModuleName("kyma-project.io/template-operator", kymaName, moduleName)

	depFeature := features.New("default").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("lifecycle manager deployment available", deploymentAvailable(KCPNS, deploymentName)).
		Assess("module operator available", deploymentAvailable(moduleOperatorNS, moduleOperatorName)).
		Assess("kyma readiness", kymaReady(KymaCRNS, kymaName)).
		Assess("manifest synced resources exists", manifestSyncedResources(KymaCRNS, manifestName)).
		Assess("module CR exists", resourceExists(KymaCRNS, manifestName, moduleCRName)).
		Feature()

	TestEnv.Test(t, depFeature)
}

func TestDefaultControllerManagerModuleUpgrade(t *testing.T) {
	moduleDeploymentName := "template-operator-v2-controller-manager"
	newChannel := "fast"
	manifestName := common.CreateModuleName("kyma-project.io/template-operator", kymaName, moduleName)

	depFeature := features.New("module upgrade").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("switch module to fast channel", switchModuleChannel(KymaCRNS, kymaName, newChannel)).
		Assess("module operator available", deploymentAvailable(moduleOperatorNS, moduleDeploymentName)).
		Assess("manifest synced resources exists", manifestSyncedResources(KymaCRNS, manifestName)).
		Assess("kyma readiness", kymaReady(KymaCRNS, kymaName)).
		Feature()

	TestEnv.Test(t, depFeature)
}

// nolint:
func TestDefaultControllerManagerKymaDelete(t *testing.T) {
	moduleDeploymentName := "template-operator-v2-controller-manager"
	moduleCRDName := fmt.Sprintf("%s.%s", strings.ToLower(moduleCRKind)+"s", v1beta2.GroupVersion.Group)
	depFeature := features.New("kyma delete").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("module CRD exists", moduleCRDExists(moduleCRDName)).
		Assess("delete Kyma", deleteKyma(KymaCRNS, kymaName)).
		Assess("module operator deleted", deploymentDeleted(moduleOperatorNS, moduleDeploymentName)).
		Assess("module CR deleted", moduleCRDeleted(KymaCRNS, moduleCRName)).
		Assess("module CRD deleted", moduleCRDDeleted(moduleCRDName)).
		Assess("kyma deleted", kymaDeleted(KymaCRNS, kymaName)).
		Feature()

	TestEnv.Test(t, depFeature)
}

func TestControlPlaneControllerManagerSpinsUp(t *testing.T) {
	deploymentName := "klm-controller-manager"
	depFeature := features.New("control-plane").
		WithLabel("app.kubernetes.io/component", "lifecycle-manager.kyma-project.io").
		WithLabel("test-type.kyma-project.io", "smoke").
		Assess("available", deploymentAvailable(KCPNS, deploymentName)).
		Assess("kyma readiness", kymaReady(KymaCRNS, kymaName)).
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

func deleteKyma(namespace string, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		kyma := getKyma(ctx, t, restConfig, name, namespace)
		if err := wait.For(func(ctx context.Context) (bool, error) {
			if err := restConfig.Delete(ctx, &kyma); err != nil {
				t.Fatal(err)
			}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}

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
	if err := wait.For(func(ctx context.Context) (bool, error) {
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
	if err := apiextensions.AddToScheme(resourcesFromConfig.GetScheme()); err != nil {
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
	if err := wait.For(func(ctx context.Context) (bool, error) {
		if err := resourcesFromConfig.Get(ctx, name, namespace, &manifest); err != nil {
			t.Fatal(err)
		}
		return manifest.Status.State == declarative.StateReady, nil
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
		if err := wait.For(func(ctx context.Context) (bool, error) {
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

func moduleCRDeleted(namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		if err := wait.For(func(ctx context.Context) (bool, error) {
			obj := builder.NewModuleCRBuilder().WithNamespace(namespace).Build()
			err := restConfig.Get(ctx, name, namespace, obj)
			if util.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("mdoule CR (%s/%s): %w", namespace, name, ErrNotDeleted)
		}); err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}

func kymaDeleted(namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		if err := wait.For(func(ctx context.Context) (bool, error) {
			err := restConfig.Get(ctx, name, namespace, &v1beta2.Kyma{})
			if util.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("kyma CR (%s/%s): %w", namespace, name, ErrNotDeleted)
		}); err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}

func moduleCRDDeleted(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		if err := wait.For(func(ctx context.Context) (bool, error) {
			err := restConfig.Get(ctx, name, "", &apiextensions.CustomResourceDefinition{})
			if util.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("module CRD (%s): %w", name, ErrNotDeleted)
		}); err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}

func moduleCRDExists(name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		if err := wait.For(func(ctx context.Context) (bool, error) {
			err := restConfig.Get(ctx, name, "", &apiextensions.CustomResourceDefinition{})
			if err != nil {
				t.Fatal(err)
			}
			return true, nil
		}); err != nil {
			t.Fatal(err)
		}
		return ctx
	}
}

func deploymentDeleted(namespace, name string) features.Func {
	return func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		t.Helper()
		restConfig := getRestConfig(t, cfg)
		var deployment apiapps.Deployment
		if err := wait.For(func(ctx context.Context) (bool, error) {
			err := restConfig.Get(ctx, name, namespace, &deployment)
			if util.IsNotFound(err) {
				return true, nil
			}
			return false, fmt.Errorf("deployment (%s/%s): %w", namespace, name, ErrNotDeleted)
		}); err != nil {
			t.Fatal(err)
		}
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
		dep := Deployment(namespace, name)
		// wait for the deployment to finish becoming available
		err = wait.For(
			conditions.New(client.Resources()).DeploymentConditionMatch(
				dep.DeepCopy(),
				apiapps.DeploymentAvailable, apicore.ConditionTrue,
			),
			wait.WithTimeout(time.Minute*3),
		)
		if err != nil {
			t.Fatal(err)
		}

		pods := apicore.PodList{}
		_ = client.Resources(namespace).List(ctx, &pods, func(options *apimachinerymeta.ListOptions) {
			sel, err := apimachinerymeta.LabelSelectorAsSelector(dep.Spec.Selector)
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

func logDeployStatus(ctx context.Context, t *testing.T, client klient.Client, dep apiapps.Deployment) {
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

func Deployment(namespace string, name string) apiapps.Deployment {
	return apiapps.Deployment{
		ObjectMeta: apimachinerymeta.ObjectMeta{
			Name: name, Namespace: namespace,
		},
	}
}
