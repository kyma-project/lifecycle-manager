package event

import (
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SKR Runtime event parsing errors
var (
	ErrInvalidEventObject   = errors.New("invalid event object type")
	ErrMissingOwner         = errors.New("missing owner in event")
	ErrMissingWatched       = errors.New("missing watched in event")
	ErrInvalidOwnerFormat   = errors.New("invalid owner format")
	ErrInvalidWatchedFormat = errors.New("invalid watched format")
)

// ExtractOwnerKey extracts the owner ObjectKey from a runtime watcher event
// This is used by controllers to determine which object to reconcile
func ExtractOwnerKey(eventObj *unstructured.Unstructured) (client.ObjectKey, error) {
	ownerData, found := eventObj.Object["owner"]
	if !found {
		return client.ObjectKey{}, ErrMissingOwner
	}

	owner, ok := ownerData.(map[string]interface{})
	if !ok {
		return client.ObjectKey{}, ErrInvalidOwnerFormat
	}

	name, _ := owner["name"].(string)
	namespace, _ := owner["namespace"].(string)

	return client.ObjectKey{Name: name, Namespace: namespace}, nil
}

// ExtractWatchedKey extracts the watched ObjectKey from a runtime watcher event
// This can be used to get information about what object was being watched
func ExtractWatchedKey(eventObj *unstructured.Unstructured) (client.ObjectKey, error) {
	watchedData, found := eventObj.Object["watched"]
	if !found {
		return client.ObjectKey{}, ErrMissingWatched
	}

	watched, ok := watchedData.(map[string]interface{})
	if !ok {
		return client.ObjectKey{}, ErrInvalidWatchedFormat
	}

	name, _ := watched["name"].(string)
	namespace, _ := watched["namespace"].(string)

	return client.ObjectKey{Name: name, Namespace: namespace}, nil
}

