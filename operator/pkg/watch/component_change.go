package watch

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const (
	Status = "status"
	state  = "state"
)

type ComponentChangeHandler struct {
	client.Reader
	client.StatusWriter
	record.EventRecorder
}

func (h *ComponentChangeHandler) ComponentChange(
	ctx context.Context,
) func(event.UpdateEvent, workqueue.RateLimitingInterface) {
	return func(event event.UpdateEvent, _ workqueue.RateLimitingInterface) {
		objectBytes, err := json.Marshal(event.ObjectNew)
		if err != nil {
			return
		}
		componentObj := unstructured.Unstructured{}
		if err = json.Unmarshal(objectBytes, &componentObj); err != nil {
			return
		}
		if componentObj.Object[Status] == nil {
			return
		}
		for key, value := range componentObj.Object[Status].(map[string]interface{}) {
			if key == state && value == string(operatorv1alpha1.KymaStateReady) {
				switch value {
				case string(operatorv1alpha1.KymaStateReady):
					h.ProcessComponentReady(ctx, event, &componentObj)
				}
			}
		}
	}
}

func (h *ComponentChangeHandler) ProcessComponentReady(ctx context.Context, event event.UpdateEvent, c *unstructured.Unstructured) {
	ownerRefs := c.GetOwnerReferences()
	var ownerName string
	kymaObj := &operatorv1alpha1.Kyma{}
	for _, ownerRef := range ownerRefs {
		if operatorv1alpha1.KymaKind == ownerRef.Kind {
			ownerName = ownerRef.Name
			break
		}
	}

	if err := h.Get(ctx, client.ObjectKey{
		Name:      ownerName,
		Namespace: c.GetNamespace(),
	}, kymaObj); err != nil {
		return
	}

	componentNameLabel := c.GetLabels()[labels.ControllerName]
	if componentNameLabel == "" {
		return
	}

	kymaStatus := status.Kyma{StatusWriter: h.StatusWriter, EventRecorder: h.EventRecorder}

	kymaStatus.UpdateReadyCondition(kymaObj, []string{componentNameLabel}, operatorv1alpha1.ConditionStatusTrue,
		fmt.Sprintf("successfully installed component : %s", event.ObjectNew.GetObjectKind().GroupVersionKind().String()))

	// TODO: "istio", "serverless" are hard-coded, remove!
	kymaStatus.UpdateReadyCondition(kymaObj, []string{"istio", "serverless"}, operatorv1alpha1.ConditionStatusTrue,
		"successfully installed component : MOCKED INJECT")

	// triggers reconciliation on Kyma
	if err := kymaStatus.UpdateStatus(ctx, kymaObj, kymaObj.Status.State,
		fmt.Sprintf("component %s set to %s state", componentNameLabel, string(operatorv1alpha1.KymaStateReady))); err != nil {
		return
	}
}
