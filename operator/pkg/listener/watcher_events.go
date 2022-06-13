package listener

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/labels"
	"github.com/kyma-project/kyma-operator/operator/pkg/release"
	"github.com/kyma-project/kyma-operator/operator/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type WatcherEventsHandler struct {
	client.Client
	logger logr.Logger
}

func (h *WatcherEventsHandler) ProcessWatcherEvent(ctx context.Context) func(event.GenericEvent, workqueue.RateLimitingInterface) {
	h.logger = log.FromContext(ctx).WithName("skr-watcher-events-processing")
	return func(genericEvent event.GenericEvent, q workqueue.RateLimitingInterface) {
		//Label component CR with a current timestamp
		//when an event is received from the SKR watcher
		componentName := genericEvent.Object.GetName()
		clusterName := genericEvent.Object.GetClusterName()
		h.logger.WithValues(
			"component", componentName,
			"cluster-name", clusterName,
		).Info("started dispatching event")

		kymaCR, err := h.getKymaCR(ctx, clusterName, componentName)
		if err != nil {
			h.logger.WithValues(
				"component", componentName,
				"cluster-name", clusterName,
			).Error(err, "failed to get Kyma CR for component")
			return
		}

		componentCR, err := h.getComponentCR(ctx, kymaCR, componentName, clusterName, q)
		if err != nil {
			h.logger.WithValues(
				"component", componentName,
				"cluster-name", clusterName,
			).Error(err, "failed to get component CR")
			return
		}
		err = h.updateComponentCRLabels(ctx, componentCR, kymaCR, q)
		if err != nil {
			h.logger.WithValues(
				"component", componentName,
				"cluster-name", clusterName,
			).Error(err, "could not trigger the reconciliation loop")
			return
		}

		h.logger.WithValues(
			"component", componentName,
			"cluster-name", clusterName,
		).Info("successfully triggered component reconciliation loop")
	}
}

func (h *WatcherEventsHandler) updateComponentCRLabels(ctx context.Context, componentCR *unstructured.Unstructured, kymaCR *v1alpha1.Kyma, q workqueue.RateLimitingInterface) error {
	componentLabels := componentCR.GetLabels()
	labelValue := strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339), ":", ".")
	componentLabels[labels.ListenerLastUpdated] = labelValue
	componentCR.SetLabels(componentLabels)
	err := h.Update(ctx, componentCR, &client.UpdateOptions{})
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("could not update component CR: %s", err)
	}
	if errors.IsNotFound(err) {
		componentKey := client.ObjectKeyFromObject(componentCR)
		err = h.handleComponentNotFoundError(ctx, kymaCR, componentKey, q)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *WatcherEventsHandler) getComponentCR(ctx context.Context, kymaCR *v1alpha1.Kyma, componentName, clusterName string, q workqueue.RateLimitingInterface) (*unstructured.Unstructured, error) {
	templates, err := release.GetTemplates(ctx, h.Client, kymaCR)
	if err != nil {
		return nil, err
	}

	componentCR, err := util.GetUnstructuredComponentFromTemplate(templates, componentName, kymaCR)
	if err != nil {
		return nil, err
	}

	componentKey := client.ObjectKeyFromObject(componentCR)
	err = h.Get(ctx, componentKey, componentCR)
	if client.IgnoreNotFound(err) != nil {
		return nil, fmt.Errorf("could not get component CR: %s", err)
	}
	if errors.IsNotFound(err) {
		err = h.handleComponentNotFoundError(ctx, kymaCR, componentKey, q)
		if err != nil {
			return nil, err
		}
	}
	return componentCR, nil
}

func (h *WatcherEventsHandler) getKymaCR(ctx context.Context, clusterName, componentName string) (*v1alpha1.Kyma, error) {
	kymaCRsForCluster := &v1alpha1.KymaList{}
	err := h.List(ctx, kymaCRsForCluster, client.MatchingLabels{
		labels.ClusterName: clusterName,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get Kyma CR for cluster: %s", err)
	}
	if len(kymaCRsForCluster.Items) == 0 {
		return nil, fmt.Errorf("kyma CR for cluster not found")
	}
	if len(kymaCRsForCluster.Items) > 1 {
		return nil, fmt.Errorf("found more than one Kyma CR for cluster")
	}
	return &kymaCRsForCluster.Items[0], nil
}

func (h *WatcherEventsHandler) handleComponentNotFoundError(ctx context.Context, kymaCR *v1alpha1.Kyma, componentKey client.ObjectKey, q workqueue.RateLimitingInterface) error {
	latestKymaCR := &v1alpha1.Kyma{}
	if err := h.Get(ctx, client.ObjectKeyFromObject(kymaCR), latestKymaCR); err != nil {
		return fmt.Errorf("could not get updated Kyma CR for component: %s", err)
	}

	if kymaCR.Status.State != v1alpha1.KymaStateProcessing {
		//this means Kyma CR is reconciled and component CR is still not created
		q.Add(reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(kymaCR),
		})
	}
	return nil
}
