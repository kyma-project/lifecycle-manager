package manifest

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

type RESTConfigGetter func() (*rest.Config, error)

type RemoteClusterLookup struct {
	KCP          *skrclient.ClusterInfo
	ConfigGetter RESTConfigGetter
}

var errTypeAssertManifest = errors.New("value can not be converted to v1beta2.Manifest")

func (r *RemoteClusterLookup) ConfigResolver(
	ctx context.Context, obj declarativev2.Object,
) (*skrclient.ClusterInfo, error) {
	manifest, ok := obj.(*v1beta2.Manifest)
	if !ok {
		return nil, errTypeAssertManifest
	}

	kymaOwnerLabel, err := internal.GetResourceLabel(manifest, shared.KymaName)
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
				ctx, kymaOwnerLabel, shared.KymaName, manifest.GetNamespace(),
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

	return &skrclient.ClusterInfo{Config: config}, nil
}
