package controllers

import (
	"flag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

func GetConfig() (*rest.Config, error) {
	// in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, err
	}

	// kubeconfig flag
	if flag.Lookup("kubeconfig") != nil {
		kubeconfig := flag.Lookup("kubeconfig").Value.String()
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	// env variable
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("masterURL", os.Getenv("KUBECONFIG"))
	}

	// If no in-cluster config, try the default location in the user's home directory
	if home := homedir.HomeDir(); home != "" {
		return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	}

	return nil, err
}
