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
	client.StatusWriter
	record.EventRecorder
}

func (h *ComponentChangeHandler) ComponentChange(ctx context.Context) func(event.UpdateEvent, workqueue.RateLimitingInterface) {
	logger := log.FromContext(ctx).WithName("component-change-handler")
	return func(event event.UpdateEvent, q workqueue.RateLimitingInterface) {
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

		if componentNew.Object[Status] == nil || componentOld.Object[Status] == nil {
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

		oldState, ok := componentOld.Object[Status].(map[string]interface{})[State]
		if !ok {
			logger.Error(errors.New("state from old component object could not be interpreted"), "missing state")
		}
		newState, ok := componentNew.Object[Status].(map[string]interface{})[State]
		if !ok {
			logger.Error(errors.New("state from new component object could not be interpreted"), "missing state")
		}

		if oldState.(string) == newState.(string) {
			return
		}

		q.Add(reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(kyma),
		})
	}
}

func (h *ComponentChangeHandler) GetKymaOwner(ctx context.Context, component *unstructured.Unstructured) (*operatorv1alpha1.Kyma, error) {
	ownerRefs := component.GetOwnerReferences()
	var ownerName string
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

	//event.ObjectNew.GetObjectKind().GroupVersionKind().String()
	return kyma, err
}
