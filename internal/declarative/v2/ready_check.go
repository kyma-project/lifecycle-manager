package v2

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var ErrNotValidClientObject = errors.New("object in resource info is not a valid client object")

type StateInfo struct {
	State
	Info string
}

type ReadyCheck interface {
	Run(ctx context.Context, clnt Client, obj Object, resources []*resource.Info) (StateInfo, error)
}

func NewExistsReadyCheck() ReadyCheck {
	return &ExistsReadyCheck{}
}

type ExistsReadyCheck struct{}

func (c *ExistsReadyCheck) Run(
	ctx context.Context,
	clnt Client,
	_ Object,
	resources []*resource.Info,
) (StateInfo, error) {
	for i := range resources {
		obj, ok := resources[i].Object.(client.Object)
		if !ok {
			return StateInfo{State: StateError}, ErrNotValidClientObject
		}
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return StateInfo{State: StateError}, fmt.Errorf("failed to fetch object by key: %w", err)
		}
	}
	return StateInfo{State: StateReady}, nil
}

/*
func NewDynamicReadyCheck() *DynamicReadyCheck {
	return &DynamicReadyCheck{
		objState: map[string]State{},
	}
}

type DynamicReadyCheck struct {
	objState map[string]State
	mut      sync.Mutex
}

func (drc *DynamicReadyCheck) Get(key string) State {
	drc.mut.Lock()
	defer drc.mut.Unlock()
	return drc.objState[key]
}

func (drc *DynamicReadyCheck) Set(key string, value State) {
	drc.mut.Lock()
	defer drc.mut.Unlock()
	drc.objState[key] = value
}

func (drc *DynamicReadyCheck) Delete(key string) {
	drc.mut.Lock()
	defer drc.mut.Unlock()
	delete(drc.objState, key)
}

func (drc *DynamicReadyCheck) Run(
	ctx context.Context,
	clnt Client,
	obj Object,
	resources []*resource.Info,
) (StateInfo, error) {

	manifest := obj.(*v1beta2.Manifest)

	objKeys := []string{}
	for i := range resources {
		obj, ok := resources[i].Object.(client.Object)
		if !ok {
			return StateInfo{State: StateError}, ErrNotValidClientObject
		}
		if err := clnt.Get(ctx, client.ObjectKeyFromObject(obj), obj); client.IgnoreNotFound(err) != nil {
			return StateInfo{State: StateError}, fmt.Errorf("failed to fetch object by key: %w", err)
		}
		objKey := obj.GetNamespace() + "/" + obj.GetName()
		objKeys = append(objKeys, objKey)
	}

	fmt.Println("========================================")
	for _, objKey := range objKeys {

		fmt.Println("Found object: ", objKey)
		stateVal := drc.Get(objKey)
		if stateVal != "" {
			fmt.Println("Found state: ", stateVal)
			if !stateVal.IsSupportedState() {
				return StateInfo{State: StateError}, fmt.Errorf("Invalid state value: %s for object: %s", stateVal, objKey)
			}
			if stateVal != StateReady {
				//First object with state different from Ready "wins"
				return StateInfo{State: stateVal}, nil
			}
		}
	}

	return StateInfo{State: StateReady}, nil
}
*/
