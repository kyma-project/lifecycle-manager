package watch

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ByField interface {
	Index(ctx context.Context) error
	Watch(ctx context.Context) handler.MapFunc
	Field() string
}

func NewKymaChannelWatch(mgr controllerruntime.Manager) ByField {
	return &KymaChannelWatch{
		indexer: mgr.GetFieldIndexer(),
		reader:  mgr.GetClient(),
		field:   "spec.channel",
	}
}

type KymaChannelWatch struct {
	reader  client.Reader
	indexer client.FieldIndexer
	field   string
}

func (c *KymaChannelWatch) Watch(ctx context.Context) handler.MapFunc {
	return func(object client.Object) []reconcile.Request {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		l := object.GetLabels()

		if errors.IsNotFound(c.reader.Get(ctx, client.ObjectKeyFromObject(object), &unstructured.Unstructured{})) {
			return []reconcile.Request{}
		}

		managedBy, managedByPresent := l[labels.ManagedBy]
		controller, controllerLabelPresent := l[labels.ControllerName]
		channel, channelPresent := l[labels.Channel]
		if !controllerLabelPresent || controller == "" || !managedByPresent || managedBy != "kyma-operator" || !channelPresent {
			return []reconcile.Request{}
		}

		affectedKymas := &operatorv1alpha1.KymaList{}

		listOps := &client.ListOptions{FieldSelector: fields.OneTermEqualSelector(c.Field(), channel)}
		err := c.reader.List(ctx, affectedKymas, listOps)
		if err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(affectedKymas.Items))
		for i, item := range affectedKymas.Items {

			componentReferencesControllerConfigMap := false
			for _, component := range item.Spec.Components {
				if component.Name == controller {
					componentReferencesControllerConfigMap = true
					break
				}
			}
			if !componentReferencesControllerConfigMap {
				continue
			}

			if err := c.reader.Get(ctx, client.ObjectKey{
				Namespace: item.Namespace,
				Name:      item.Name,
			}, &unstructured.Unstructured{}); errors.IsNotFound(err) {
				continue
			}

			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      fmt.Sprintf(item.GetName()),
					Namespace: item.GetNamespace(),
				},
			}
		}
		return requests
	}
}

func (c *KymaChannelWatch) Field() string {
	return c.field
}

func (c *KymaChannelWatch) Index(ctx context.Context) error {
	if err := c.indexer.IndexField(ctx, &operatorv1alpha1.Kyma{}, c.Field(), func(object client.Object) []string {
		kyma := object.(*operatorv1alpha1.Kyma)
		if kyma.Spec.Channel == "" {
			return nil
		}
		return []string{string(kyma.Spec.Channel)}
	}); err != nil {
		return err
	}
	return nil
}
