package internal

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

type k3dContextKey string

// CreateKymaK3dCluster returns an env.Func that is used to
// create a k3d cluster that is then injected in the context
// using the name as a key.
//
// NOTE: the returned function will update its env config with the
// kubeconfig file for the config client.
func CreateKymaK3dCluster(clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		log.Println("Setting up Kyma CLI")
		if err := SetupKymaCLI(); err != nil {
			return ctx, err
		}

		provArgs := []string{"provision", "k3d", "--name", clusterName,
			"-p", "8083:80@loadbalancer",
			"-p", "8443:443@loadbalancer",
			"--timeout", "1m",
		}
		log.Printf("Provisioning Cluster with %s\n", provArgs)
		provision := KymaCLI(provArgs...)
		if err := provision.Run(); err != nil {
			return nil, err
		}

		labels := cfg.Labels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels["provider.kyma-project.io"] = "kyma-cli"
		labels["test-type.kyma-project.io"] = "smoke"
		cfg.WithLabels(labels)

		kubeconfigFile := filepath.Join(os.TempDir(), "kubeconfig-kyma")
		log.Println("Merging Kubeconfigs")
		kubeconfigSync := exec.Command("k3d", "kubeconfig", "merge", clusterName, "-o", kubeconfigFile)
		if err := kubeconfigSync.Run(); err != nil {
			return nil, err
		}

		// update envconfig  with kubeconfig
		cfg.WithKubeconfigFile(kubeconfigFile)

		// store entire cluster value in ctx for future access using the cluster name
		return ctx, nil
	}
}

// DestroyKymaK3dCluster returns an EnvFunc that
// retrieves a previously saved Cluster through k3d, then deletes it and its registry by a naming convention.
//
// NOTE: this should be used in a Environment.Finish step.
func DestroyKymaK3dCluster(clusterName string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		clusterDelete := exec.Command("k3d", "cluster", "delete", clusterName)
		if err := clusterDelete.Run(); err != nil {
			return nil, err
		}
		registryDelete := exec.Command("k3d", "registry", "delete", clusterName+"-registry")
		if err := registryDelete.Run(); err != nil {
			return nil, err
		}
		return ctx, nil
	}
}

func InstallWithKustomize(kustomizeDir string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		log.Printf("Creating kustomize resources")
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			return ctx, err
		}
		log.Printf("Setting up kustomize")
		if err := SetupKustomize(); err != nil {
			return ctx, err
		}
		log.Printf("Building with kustomize")
		manifests, err := BuildWithKustomize(kustomizeDir)
		if err != nil {
			return ctx, err
		}
		// decode and create a stream of YAML or JSON documents from an io.Reader
		if err := decoder.DecodeEach(ctx, bytes.NewReader(manifests), decoder.CreateHandler(r)); err != nil {
			return ctx, err
		}
		log.Printf("Finished building with kustomize")
		return ctx, nil
	}
}
