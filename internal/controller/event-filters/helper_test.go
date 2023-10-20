package event_filters_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/kyma-project/lifecycle-manager/api/shared"
	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	errKymaNotInExpectedState   = errors.New("kyma is not in expected state")
	errKymaNotInExpectedChannel = errors.New("kyma doesn't have expected channel")
	errKymaNotHaveExpectedLabel = errors.New("kyma doesn't have expected label")
)

func updateKymaChannel(ctx context.Context,
	k8sClient client.Client,
	kyma *v1beta2.Kyma,
	channel string,
) error {
	kyma.Spec.Channel = channel

	return updateKyma(ctx, k8sClient, kyma)
}

func addLabelToKyma(ctx context.Context,
	k8sClient client.Client,
	kyma *v1beta2.Kyma,
	labelKey, labelValue string,
) error {
	if kyma.Labels == nil {
		kyma.Labels = make(map[string]string)
	}
	kyma.Labels[labelKey] = labelValue

	return updateKyma(ctx, k8sClient, kyma)
}

func kymaIsInExpectedStateWithUpdatedChannel(k8sClient client.Client,
	kymaName string,
	kymaNamespace string,
	expectedChannel string,
	expectedState State,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}

	if kyma.Spec.Channel != expectedChannel || kyma.Status.ActiveChannel != expectedChannel {
		return fmt.Errorf("%w: expected channel: %s, but found: %s",
			errKymaNotInExpectedChannel, expectedChannel, kyma.Spec.Channel)
	}

	return validateKymaStatus(kyma.Status.State, expectedState)
}

func kymaIsInExpectedStateWithLabelUpdated(k8sClient client.Client,
	kymaName string,
	kymaNamespace string,
	expectedLabelKey string,
	expectedLabelValue string,
	expectedState State,
) error {
	kyma := &v1beta2.Kyma{}
	if err := k8sClient.Get(ctx, client.ObjectKey{Name: kymaName, Namespace: kymaNamespace}, kyma); err != nil {
		return err
	}

	if kyma.Labels[expectedLabelKey] != expectedLabelValue {
		return fmt.Errorf("%w: expected label value: %s, but found: %s",
			errKymaNotHaveExpectedLabel, expectedLabelValue, kyma.Labels[expectedLabelKey])
	}

	return validateKymaStatus(kyma.Status.State, expectedState)
}

func validateKymaStatus(kymaState, expectedState State) error {
	if kymaState != expectedState {
		return fmt.Errorf("%w: expected state: %s, but found: %s",
			errKymaNotInExpectedState, expectedState, kymaState)
	}

	return nil
}

func updateKyma(ctx context.Context, k8sClient client.Client, kyma *v1beta2.Kyma) error {
	err := k8sClient.Update(ctx, kyma)
	if err != nil {
		return fmt.Errorf("failed to update Kyma with error %w", err)
	}
	return nil
}
