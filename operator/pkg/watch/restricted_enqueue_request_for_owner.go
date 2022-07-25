package watch

import (
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const (
	Status = "status"
	State  = "state"
)

var ErrStateInvalid = errors.New("state from component object could not be interpreted")

// EnqueueRequestForOwner enqueues Requests for the Owners of an object.  E.g. the object that created
// the object that was the source of the Event.
//
// If a ReplicaSet creates Pods, users may reconcile the ReplicaSet in response to Pod Events using:
//
// - a source.Kind Source with Type of Pod.
//
// - a handler.EnqueueRequestForOwner EventHandler with an OwnerType of ReplicaSet and IsController set to true.
type RestrictedEnqueueRequestForOwner struct {
	Log logr.Logger

	// OwnerType is the type of the Owner object to look for in OwnerReferences.  Only Group and Kind are compared.
	OwnerType runtime.Object

	// IsController if set will only look at the first OwnerReference with Controller: true.
	IsController bool

	// groupKind is the cached Group and Kind from OwnerType
	groupKind schema.GroupKind

	// mapper maps GroupVersionKinds to Resources
	mapper meta.RESTMapper
}

// Create implements EventHandler.
func (e *RestrictedEnqueueRequestForOwner) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]any{}
	e.getOwnerReconcileRequest(nil, evt.Object, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// Update implements EventHandler.
func (e *RestrictedEnqueueRequestForOwner) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]any{}
	e.getOwnerReconcileRequest(evt.ObjectOld, evt.ObjectNew, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// Delete implements EventHandler.
func (e *RestrictedEnqueueRequestForOwner) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]any{}
	e.getOwnerReconcileRequest(nil, evt.Object, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// Generic implements EventHandler.
func (e *RestrictedEnqueueRequestForOwner) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	reqs := map[reconcile.Request]any{}
	e.getOwnerReconcileRequest(nil, evt.Object, reqs)
	for req := range reqs {
		q.Add(req)
	}
}

// parseOwnerTypeGroupKind parses the OwnerType into a Group and Kind and caches the result.  Returns false
// if the OwnerType could not be parsed using the scheme.
func (e *RestrictedEnqueueRequestForOwner) parseOwnerTypeGroupKind(scheme *runtime.Scheme) error {
	// Get the kinds of the type
	kinds, _, err := scheme.ObjectKinds(e.OwnerType)
	if err != nil {
		e.Log.Error(err, "Could not get ObjectKinds for OwnerType", "owner type", fmt.Sprintf("%T", e.OwnerType))
		return err
	}
	// Expect only 1 kind.  If there is more than one kind this is probably an edge case such as ListOptions.
	if len(kinds) != 1 {
		err := fmt.Errorf("expected exactly 1 kind for OwnerType %T, but found %s kinds", e.OwnerType, kinds)
		e.Log.Error(nil, "expected exactly 1 kind for OwnerType", "owner type",
			fmt.Sprintf("%T", e.OwnerType), "kinds", kinds)
		return err
	}
	// Cache the Group and Kind for the OwnerType
	e.groupKind = schema.GroupKind{Group: kinds[0].Group, Kind: kinds[0].Kind}
	return nil
}

// getOwnerReconcileRequest looks at object and builds a map of reconcile.Request to reconcile
// owners of object that match e.OwnerType.
func (e *RestrictedEnqueueRequestForOwner) getOwnerReconcileRequest(
	oldIfAny client.Object, object client.Object, result map[reconcile.Request]any,
) {
	// Iterate through the OwnerReferences looking for a match on Group and Kind against what was requested
	// by the user
	for _, ref := range e.getOwnersReferences(object) {
		// Parse the Group out of the OwnerReference to compare it to what was parsed out of the requested OwnerType
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			e.Log.Error(err, "Could not parse OwnerReference APIVersion",
				"api version", ref.APIVersion)
			return
		}
		e.getOwnerReconcileRequestFromOwnerReference(oldIfAny, object, result, ref, refGV)
	}
}

func (e *RestrictedEnqueueRequestForOwner) getOwnerReconcileRequestFromOwnerReference(
	oldIfAny client.Object, object client.Object,
	result map[reconcile.Request]any,
	ref metav1.OwnerReference,
	refGV schema.GroupVersion,
) {
	// Compare the OwnerReference Group and Kind against the OwnerType Group and Kind specified by the user.
	// If the two match, create a Request for the objected referred to by
	// the OwnerReference.  Use the Name from the OwnerReference and the Namespace from the
	// object in the event.
	if ref.Kind != e.groupKind.Kind || refGV.Group != e.groupKind.Group {
		return
	}

	// Match found - add a Request for the object referred to in the OwnerReference
	request := reconcile.Request{NamespacedName: types.NamespacedName{
		Name: ref.Name,
	}}

	// if owner is not namespaced then we should set the namespace to the empty
	mapping, err := e.mapper.RESTMapping(e.groupKind, refGV.Version)
	if err != nil {
		e.Log.Error(err, "Could not retrieve rest mapping", "kind", e.groupKind)
		return
	}
	if mapping.Scope.Name() != meta.RESTScopeNameRoot {
		request.Namespace = object.GetNamespace()
	}

	if oldIfAny != nil {
		componentOld, okOld := oldIfAny.(*unstructured.Unstructured)
		componentNew, okNew := object.(*unstructured.Unstructured)

		if err != nil || !okNew || !okOld {
			e.Log.Error(err, "error getting owner")
		}

		// In case component state can't be updated due to operator error, still possible enqueue by Generation update.
		if componentOld.GetGeneration() != componentNew.GetGeneration() {
			result[request] = ref
		}
		oldState := extractState(componentOld, e.Log)
		newState := extractState(componentNew, e.Log)

		if oldState.(string) != newState.(string) {
			result[request] = ref
		}
		return
	}

	result[request] = ref
}

// getOwnersReferences returns the OwnerReferences for an object as specified by the EnqueueRequestForOwner
// - if IsController is true: only take the Controller OwnerReference (if found)
// - if IsController is false: take all OwnerReferences.
func (e *RestrictedEnqueueRequestForOwner) getOwnersReferences(object metav1.Object) []metav1.OwnerReference {
	if object == nil {
		return nil
	}

	// If not filtered as Controller only, then use all the OwnerReferences
	if !e.IsController {
		return object.GetOwnerReferences()
	}
	// If filtered to a Controller, only take the Controller OwnerReference
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		return []metav1.OwnerReference{*ownerRef}
	}
	// No Controller OwnerReference found
	return nil
}

var _ inject.Scheme = &RestrictedEnqueueRequestForOwner{}

// InjectScheme is called by the Controller to provide a singleton scheme to the EnqueueRequestForOwner.
func (e *RestrictedEnqueueRequestForOwner) InjectScheme(s *runtime.Scheme) error {
	return e.parseOwnerTypeGroupKind(s)
}

var _ inject.Mapper = &RestrictedEnqueueRequestForOwner{}

// InjectMapper  is called by the Controller to provide the rest mapper used by the manager.
func (e *RestrictedEnqueueRequestForOwner) InjectMapper(m meta.RESTMapper) error {
	e.mapper = m
	return nil
}

func extractState(component *unstructured.Unstructured, logger logr.Logger) interface{} {
	var state interface{}

	var ok bool

	if component.Object[Status] != nil {
		state, ok = component.Object[Status].(map[string]interface{})[State]
		if !ok {
			logger.Error(ErrStateInvalid, "missing state")
		}
	} else {
		state = ""
	}

	return state
}
