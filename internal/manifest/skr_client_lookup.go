package manifest

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/internal"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/internal/service/accessmanager"
	"github.com/kyma-project/lifecycle-manager/internal/service/skrclient"
)

type RemoteClusterLookup struct {
	KCP                  *skrclient.ClusterInfo
	accessManagerService *accessmanager.Service
}

func NewRemoteClusterLookup(kcp *skrclient.ClusterInfo,
	accessManagerService *accessmanager.Service,
) *RemoteClusterLookup {
	return &RemoteClusterLookup{
		KCP:                  kcp,
		accessManagerService: accessManagerService,
	}
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

	config, err := r.accessManagerService.GetAccessRestConfigByKyma(ctx, kymaOwnerLabel)
	if err != nil {
		return nil, err
	}

	config.QPS = r.KCP.Config.QPS
	config.Burst = r.KCP.Config.Burst

	return &skrclient.ClusterInfo{Config: config}, nil
}
