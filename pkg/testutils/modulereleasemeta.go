package testutils

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrNotExpectedChannelVersion = errors.New("channel-version pair not found")

func CreateModuleReleaseMeta(ctx context.Context,
	clnt client.Client,
	mrm *v1beta2.ModuleReleaseMeta,
) error {
	mrm.SetResourceVersion("") // must be reset to enable retries
	if err := clnt.Create(ctx, mrm); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("creating ModuleReleaseMeta failed: %w", err)
	}
	return nil
}

func UpdateChannelVersionInModuleReleaseMeta(ctx context.Context, clnt client.Client,
	moduleName, namespace, channel, version string,
) error {
	mrm, err := GetModuleReleaseMeta(ctx, moduleName, namespace, clnt)
	if err != nil {
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
		return nil, fmt.Errorf("failed to get ModuleReleaseMeta"+
			" with name %q in namespace %q: %w", moduleName, namespace, err)
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

func DeleteModuleReleaseMeta(ctx context.Context, moduleName, namespace string, clnt client.Client) error {
	mrm, err := GetModuleReleaseMeta(ctx, moduleName, namespace, clnt)
	if util.IsNotFound(err) {
		return nil
	}

	err = client.IgnoreNotFound(clnt.Delete(ctx, mrm))
	if err != nil {
		return fmt.Errorf("modulereleasemeta not deleted: %w", err)
	}
	return nil
}

func UpdateAllModuleReleaseMetaChannelVersions(ctx context.Context, client client.Client,
	namespace, name, version string,
) error {
	meta := &v1beta2.ModuleReleaseMeta{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, meta); err != nil {
		return err
	}
	for i := range meta.Spec.Channels {
		meta.Spec.Channels[i].Version = version
	}
	if err := client.Update(ctx, meta); err != nil {
		return err
	}
	return nil
}

func MandatoryModuleReleaseMetaHasVersion(ctx context.Context, clnt client.Client, moduleName, version string,
) error {
	mrmList := &v1beta2.ModuleReleaseMetaList{}
	if err := clnt.List(ctx, mrmList); err != nil {
		return fmt.Errorf("failed to list ModuleReleaseMeta resources: %w", err)
	}

	for _, mrm := range mrmList.Items {
		if mrm.Spec.ModuleName == moduleName {
			if mrm.Spec.Mandatory == nil {
				return fmt.Errorf("module %s is not configured as mandatory", moduleName)
			}

			if mrm.Spec.Mandatory.Version != version {
				return fmt.Errorf("mandatory module %s version mismatch: expected %s, got %s",
					moduleName, version, mrm.Spec.Mandatory.Version)
			}

			return nil
		}
	}

	return fmt.Errorf("mandatory module %s not found", moduleName)
}

// FullOCMName returns the fully qualified OCM component name for a given module name.
// This is used by OCM-related functionality, end-users do not have to use this format.
func FullOCMName(moduleName string) string {
	return shared.KymaGroup + "/module/" + moduleName
}
