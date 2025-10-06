package commontestutils

import (
	"context"
	"errors"
	"fmt"

	apiappsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/pkg/testutils"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrStatefulSetNotReady = errors.New("statefulset is not ready")

func StatefulSetIsReady(ctx context.Context, clnt client.Client, name, namespace string) error {
	statefulSet, err := GetStatefulSet(ctx, clnt, name, namespace)
	if err != nil {
		if util.IsNotFound(err) {
			return testutils.ErrNotFound
		}
		return fmt.Errorf("could not get statefulset: %w", err)
	}

	if statefulSet.Spec.Replicas != nil &&
		*statefulSet.Spec.Replicas == statefulSet.Status.ReadyReplicas {
		return nil
	}
	return ErrStatefulSetNotReady
}

func GetStatefulSet(ctx context.Context, clnt client.Client,
	name, namespace string,
) (*apiappsv1.StatefulSet, error) {
	statefulSet := &apiappsv1.StatefulSet{}
	if err := clnt.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, statefulSet); err != nil {
		return nil, fmt.Errorf("could not get statefulset: %w", err)
	}
	return statefulSet, nil
}
