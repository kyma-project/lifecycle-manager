package testutils

import (
	"context"

	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrConfigMapNotExist = errors.New("maintenance window configmap does not exist")

func MaintenanceWindowConfigMapExists(ctx context.Context, kcpClient client.Client) error {
	cm := &v1.ConfigMap{}
	objectKey := client.ObjectKey{
		Namespace: ControlPlaneNamespace,
		Name:      "maintenance-config",
	}

	if err := kcpClient.Get(ctx, objectKey, cm); err != nil {
		if util.IsNotFound(err) {
			return ErrConfigMapNotExist
		}

		return fmt.Errorf("could not get configmap: %w", err)
	}

	return nil
}
