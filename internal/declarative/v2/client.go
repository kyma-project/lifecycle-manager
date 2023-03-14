package v2

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client interface {
	kube.Factory
	Install() *action.Install
	KubeClient() *kube.Client

	resource.RESTClientGetter
	ResourceInfoConverter

	client.Client
}
