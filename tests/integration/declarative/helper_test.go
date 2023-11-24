package declarative_test

import (
	"context"
	"fmt"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/shared"
	declarativev2 "github.com/kyma-project/lifecycle-manager/internal/declarative/v2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
	declarativetest "github.com/kyma-project/lifecycle-manager/tests/integration/declarative"

	. "github.com/onsi/gomega"
)

// BeInState determines if the resource is in a given declarative state.
func BeInState(state shared.State) *BeInStateMatcher {
	return &BeInStateMatcher{state: state}
}

type BeInStateMatcher struct {
	state shared.State
}

func (matcher *BeInStateMatcher) Match(actual interface{}) (bool, error) {
	status, ok := actual.(shared.Status)
	if !ok {
		//nolint:goerr113
		return false, fmt.Errorf("Expected a Status. Got:\n%s", format.Object(actual, 1))
	}

	return status.State == matcher.state, nil
}

func (matcher *BeInStateMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, fmt.Sprintf("to be %s", matcher.state))
}

func (matcher *BeInStateMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(
		actual, fmt.Sprintf("not %s", matcher.FailureMessage(actual)),
	)
}

func HaveConditionWithStatus(
	conditionType declarativev2.ConditionType, status apimetav1.ConditionStatus,
) *HaveConditionMatcher {
	return &HaveConditionMatcher{condition: conditionType, status: status}
}

type HaveConditionMatcher struct {
	condition declarativev2.ConditionType
	status    apimetav1.ConditionStatus
}

func (matcher *HaveConditionMatcher) Match(actual interface{}) (bool, error) {
	status, ok := actual.(shared.Status)
	if !ok {
		//nolint:goerr113
		return false, fmt.Errorf("Expected a Status. Got:\n%s", format.Object(actual, 1))
	}

	return meta.IsStatusConditionPresentAndEqual(
		status.Conditions,
		string(matcher.condition),
		matcher.status,
	), nil
}

func (matcher *HaveConditionMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, fmt.Sprintf("to have condition %s with status %s", matcher.condition, matcher.status))
}

func (matcher *HaveConditionMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(
		actual, fmt.Sprintf("not %s", matcher.FailureMessage(actual)),
	)
}

func EventuallyDeclarativeStatusShould(ctx context.Context, key client.ObjectKey, testClient client.Client,
	matchers ...types.GomegaMatcher,
) {
	EventuallyWithOffset(1, StatusOnCluster).
		WithContext(ctx).
		WithArguments(key, testClient).
		WithPolling(standardInterval).
		WithTimeout(standardTimeout).
		Should(And(matchers...))
}

func EventuallyDeclarativeShouldBeUninstalled(ctx context.Context, obj *declarativetest.TestAPI, testClient client.Client) {
	EventuallyWithOffset(1, testClient.Get).
		WithContext(ctx).
		WithArguments(client.ObjectKeyFromObject(obj), &declarativetest.TestAPI{}).
		WithPolling(standardInterval).
		WithTimeout(standardTimeout).
		Should(Satisfy(util.IsNotFound))

	synced := obj.GetStatus().Synced
	for i := range synced {
		unstruct := synced[i].ToUnstructured()
		ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(unstruct), unstruct)).
			To(Satisfy(util.IsNotFound))
	}
}

// HaveAllSyncedResourcesExistingInCluster determines if all synced resources actually exist in the cluster.
func HaveAllSyncedResourcesExistingInCluster(ctx context.Context,
	testClient client.Client,
) *SyncedResourcesExistingMatcher {
	return &SyncedResourcesExistingMatcher{ctx: &ctx, testClient: testClient}
}

type SyncedResourcesExistingMatcher struct {
	ctx        *context.Context
	testClient client.Client
}

func (matcher *SyncedResourcesExistingMatcher) Match(actual interface{}) (bool, error) {
	status, ok := actual.(shared.Status)
	if !ok {
		//nolint:goerr113
		return false, fmt.Errorf("Expected a Status. Got:\n%s", format.Object(actual, 1))
	}

	ctx := matcher.ctx
	synced := status.Synced

	for i := range synced {
		unstruct := synced[i].ToUnstructured()
		if err := matcher.testClient.Get(*ctx, client.ObjectKeyFromObject(unstruct), unstruct); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (matcher *SyncedResourcesExistingMatcher) FailureMessage(actual interface{}) string {
	return format.Message(actual, "to have status with all synced resources actually existing in cluster")
}

func (matcher *SyncedResourcesExistingMatcher) NegatedFailureMessage(actual interface{}) string {
	return format.Message(actual, fmt.Sprintf("not %s", matcher.FailureMessage(actual)))
}
