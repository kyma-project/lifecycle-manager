package controllers

import (
	"flag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
)

func GetConfigPath() string {
	// in-cluster config
	_, err := rest.InClusterConfig()
	if err == nil {
		return ""
	}

	// kubeconfig flag
	if flag.Lookup("kubeconfig") != nil {
		if kubeconfig := flag.Lookup("kubeconfig").Value.String(); kubeconfig != "" {
			return kubeconfig
		}
	}

	// env variable
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return os.Getenv("KUBECONFIG")
	}

	// If no in-cluster config, try the default location in the user's home directory
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}

	return ""
}
