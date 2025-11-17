package event

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Repository struct {
	clnt      client.Client
	namespace string
	eventName string
}

func NewRepository(clnt client.Client,
	namespace string,
	eventName string,
) *Repository {
	return &Repository{
		clnt:      clnt,
		namespace: namespace,
		eventName: eventName,
	}
}

func (r *Repository) Create(ctx context.Context,
	involvedObject corev1.ObjectReference,
	eventType, reason, message string,
) {
	event := &corev1.Event{
		Type:           eventType,
		Reason:         reason,
		Message:        message,
		InvolvedObject: involvedObject,
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s/%d", r.eventName, time.Now().UnixMilli()),
			Namespace: r.namespace,
		},
	}

	// TODO: what should we do with an error here? Should we at least log it?
	_ = r.clnt.Create(ctx, event)
}
