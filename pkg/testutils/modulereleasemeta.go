package testutils

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNotExpectedChannelVersion = errors.New("channel-version pair not found")

func UpdateChannelVersionIfModuleReleaseMetaExists(ctx context.Context, clnt client.Client,
	moduleName, namespace, channel, version string,
) error {
	mrm, err := GetModuleReleaseMeta(ctx, moduleName, namespace, clnt)
	if err != nil {
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get module release meta: %w", err)
	}

	channelFound := false
	for i, ch := range mrm.Spec.Channels {
		if ch.Channel == channel {
			mrm.Spec.Channels[i].Version = version
			channelFound = true
			break
		}
	}

	if !channelFound {
		mrm.Spec.Channels = append(mrm.Spec.Channels, v1beta2.ChannelVersionAssignment{
			Channel: channel,
			Version: version,
		})
	}

	if err = clnt.Update(ctx, mrm); err != nil {
		return fmt.Errorf("update module release meta: %w", err)
	}

	return nil
}

func GetModuleReleaseMeta(ctx context.Context, moduleName, namespace string,
	clnt client.Client,
) (*v1beta2.ModuleReleaseMeta, error) {
	mrm := &v1beta2.ModuleReleaseMeta{}

	err := clnt.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      moduleName,
	}, mrm)
	if err != nil {
		return nil, fmt.Errorf("get kyma: %w", err)
	}
	return mrm, nil
}

func ModuleReleaseMetaExists(ctx context.Context, moduleName, namespace string, clnt client.Client) error {
	if _, err := GetModuleReleaseMeta(ctx, moduleName, namespace, clnt); err != nil {
		if util.IsNotFound(err) {
			return ErrNotFound
		}
	}

	return nil
}

func ModuleReleaseMetaContainsCorrectChannelVersion(ctx context.Context,
	moduleName, namespace, channel, version string, clnt client.Client,
) error {
	mrm, err := GetModuleReleaseMeta(ctx, moduleName, namespace, clnt)
	if err != nil {
		return fmt.Errorf("failed to fetch modulereleasemeta, %w", err)
	}

	for _, ch := range mrm.Spec.Channels {
		if ch.Channel == channel {
			if ch.Version == version {
				return nil
			}
		}
	}

	return ErrNotExpectedChannelVersion
}
