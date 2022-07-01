package watch

import (
	"context"
	"errors"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	Status = "status"
	State  = "state"
)

type ComponentChangeHandler struct {
	client.Reader
	record.EventRecorder
}

func NewComponentChangeHandler(handlerClient ChangeHandlerClient) *ComponentChangeHandler {
	return &ComponentChangeHandler{Reader: handlerClient, EventRecorder: handlerClient}
}

func (h *ComponentChangeHandler) Watch(ctx context.Context) func(event.UpdateEvent, workqueue.RateLimitingInterface) {
	logger := log.FromContext(ctx).WithName("component-change-handler")

	return func(event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
		objectBytesNew, err := json.Marshal(event.ObjectNew)
		if err != nil {
			logger.Error(err, "error transforming new component object")

			return
		}

		objectBytesOld, err := json.Marshal(event.ObjectOld)
		if err != nil {
			logger.Error(err, "error transforming old component object")

			return
		}

		componentNew := unstructured.Unstructured{}
		componentOld := unstructured.Unstructured{}

		if err = json.Unmarshal(objectBytesNew, &componentNew); err != nil {
			logger.Error(err, "error transforming new component object")

			return
		}

		if err = json.Unmarshal(objectBytesOld, &componentOld); err != nil {
			logger.Error(err, "error transforming old component object")

			return
		}

		if componentNew.Object[Status] == nil {
			return
		}

		componentNameLabel := componentNew.GetLabels()[labels.ControllerName]
		if componentNameLabel == "" {
			return
		}

		kyma, err := h.GetKymaOwner(ctx, &componentNew)
		if err != nil {
			logger.Error(err, "error getting Kyma owner")
		}

		var oldState, newState interface{}

		var valueExists bool

		if componentOld.Object[Status] != nil {
			oldState, valueExists = componentOld.Object[Status].(map[string]interface{})[State]
			if !valueExists {
				logger.Error(errors.New("state from old component object could not be interpreted"), "missing state")
			}
		} else {
			oldState = ""
		}

		newState, valueExists = componentNew.Object[Status].(map[string]interface{})[State]
		if !valueExists {
			logger.Error(errors.New("state from new component object could not be interpreted"), "missing state")
		}

		if oldState.(string) == newState.(string) {
			return
		}

		queue.Add(reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(kyma),
		})
	}
}

func (h *ComponentChangeHandler) GetKymaOwner(ctx context.Context, component *unstructured.Unstructured) (*operatorv1alpha1.Kyma, error) {
	var ownerName string

	ownerRefs := component.GetOwnerReferences()
	kyma := &operatorv1alpha1.Kyma{}

	for _, ownerRef := range ownerRefs {
		if operatorv1alpha1.KymaKind == ownerRef.Kind {
			ownerName = ownerRef.Name

			break
		}
	}

	err := h.Get(ctx, client.ObjectKey{
		Name:      ownerName,
		Namespace: component.GetNamespace(),
	}, kyma)

	return kyma, err
}
