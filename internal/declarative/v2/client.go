package v2

import (
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/internal/manifest/skrresources"
)

type Client interface {
	resource.RESTClientGetter
	skrresources.ResourceInfoConverter

	client.Client
}
