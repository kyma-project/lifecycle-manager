package v2

import (
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Client interface {
	resource.RESTClientGetter
	ResourceInfoConverter

	client.Client
}
