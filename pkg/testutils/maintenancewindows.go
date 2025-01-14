package testutils

import (
	"context"
	"errors"
	"fmt"

	apicorev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrConfigMapNotExist = errors.New("maintenance window configmap does not exist")

func MaintenanceWindowConfigMapExists(ctx context.Context, kcpClient client.Client) error {
	maintenanceWindowsCM := &apicorev1.ConfigMap{}
	objectKey := client.ObjectKey{
		Namespace: ControlPlaneNamespace,
		Name:      "maintenance-config",
	}

	if err := kcpClient.Get(ctx, objectKey, maintenanceWindowsCM); err != nil {
		if util.IsNotFound(err) {
			return ErrConfigMapNotExist
		}

		return fmt.Errorf("could not get configmap: %w", err)
	}

	return nil
}
