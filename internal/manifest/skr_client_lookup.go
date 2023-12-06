package manifest

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
)

type RESTConfigGetter func() (*rest.Config, error)

type RemoteClusterLookup struct {
	KCP          *declarativev2.ClusterInfo
	ConfigGetter RESTConfigGetter
}

func (r *RemoteClusterLookup) ConfigResolver(
	ctx context.Context, obj declarativev2.Object,
) (*declarativev2.ClusterInfo, error) {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil, v1beta2.ErrTypeAssertManifest
	}
	// in single cluster mode return the default cluster info
	// since the resources need to be installed in the same cluster
	if !manifest.Spec.Remote {
		return r.KCP, nil
	}

	kymaOwnerLabel, err := internal.GetResourceLabel(manifest, v1beta2.KymaName)
	if err != nil {
		return nil, fmt.Errorf("failed to get kyma owner label: %w", err)
	}

	// RESTConfig can either be retrieved by a secret with name contained in labels.KymaName Manifest CR label,
	// or it can be retrieved as a function return value, passed during controller startup.
	var restConfigGetter RESTConfigGetter
	if r.ConfigGetter != nil {
		restConfigGetter = r.ConfigGetter
	} else {
		restConfigGetter = func() (*rest.Config, error) {
			// evaluate remote rest config from secret
			config, err := (&ClusterClient{DefaultClient: r.KCP.Client}).GetRESTConfig(
				ctx, kymaOwnerLabel, v1beta2.KymaName, manifest.GetNamespace(),
			)
			if err != nil {
				return nil, fmt.Errorf("could not resolve remote cluster rest config: %w", err)
			}
			return config, nil
		}
	}

	config, err := restConfigGetter()
	if err != nil {
		return nil, err
	}

	config.QPS = r.KCP.Config.QPS
	config.Burst = r.KCP.Config.Burst

	return &declarativev2.ClusterInfo{Config: config}, nil
}
