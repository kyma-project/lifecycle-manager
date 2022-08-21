package status

import (
	"context"
	"errors"
	"fmt"

	"github.com/kyma-project/kyma-operator/operator/pkg/parsed"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"github.com/kyma-project/kyma-operator/operator/pkg/watch"
)

var ErrTemplateNotFound = errors.New("template not found")

type Kyma struct {
	client.StatusWriter
}

type Handler interface {
	Status() client.StatusWriter
}

func Helper(handler Handler) *Kyma {
	return &Kyma{StatusWriter: handler.Status()}
}

func (k *Kyma) UpdateStatusForExistingModules(ctx context.Context,
	kyma *operatorv1alpha1.Kyma, newState operatorv1alpha1.State,
) error {
	kyma.Status.State = newState

	switch newState {
	case operatorv1alpha1.StateReady:
		kyma.SetActiveChannel()
	case "":
	case operatorv1alpha1.StateDeleting:
	case operatorv1alpha1.StateError:
	case operatorv1alpha1.StateProcessing:
	default:
	}

	if err := k.Update(ctx, kyma.SetObservedGeneration()); err != nil {
		return fmt.Errorf("status could not be updated: %w", err)
	}

	return nil
}

func (k *Kyma) SyncModuleInfo(kyma *operatorv1alpha1.Kyma, modules parsed.Modules) bool {
	moduleInfoMap := kyma.GetModuleInfoMap()
	updateRequired := false
	for _, module := range modules {
		latestModuleInfo := operatorv1alpha1.ModuleInfo{
			ModuleName: module.Name,
			Name:       module.Unstructured.GetName(),
			Namespace:  module.Unstructured.GetNamespace(),
			TemplateInfo: operatorv1alpha1.TemplateInfo{
				Channel:    module.Template.Spec.Channel,
				Generation: module.Template.Generation,
				GroupVersionKind: metav1.GroupVersionKind{
					Group:   module.GroupVersionKind().Group,
					Version: module.GroupVersionKind().Version,
					Kind:    module.GroupVersionKind().Kind,
				},
			},
			State: k.getModuleState(module.Unstructured),
		}
		moduleInfo, exists := moduleInfoMap[module.Name]
		if exists {
			if moduleInfo.State != latestModuleInfo.State {
				updateRequired = true
			}
			*moduleInfo = latestModuleInfo
		} else {
			updateRequired = true
			kyma.Status.ModuleInfos = append(kyma.Status.ModuleInfos, latestModuleInfo)
		}
	}
	return updateRequired
}

func (k *Kyma) GetTemplateInfoForModule(
	kyma *operatorv1alpha1.Kyma,
	moduleName string,
) (*operatorv1alpha1.TemplateInfo, error) {
	for i := range kyma.Status.ModuleInfos {
		moduleInfo := &kyma.Status.ModuleInfos[i]
		if moduleInfo.ModuleName == moduleName {
			return &moduleInfo.TemplateInfo, nil
		}
	}
	// should not happen
	return nil, ErrTemplateNotFound
}

func (k *Kyma) getModuleState(module *unstructured.Unstructured) operatorv1alpha1.State {
	state, found, err := unstructured.NestedString(module.Object, watch.Status, watch.State)
	if !found {
		return operatorv1alpha1.StateProcessing
	}
	if err == nil && isValidState(state) {
		return operatorv1alpha1.State(state)
	}
	return operatorv1alpha1.StateError
}

func isValidState(state string) bool {
	castedState := operatorv1alpha1.State(state)
	return castedState == operatorv1alpha1.StateReady ||
		castedState == operatorv1alpha1.StateProcessing ||
		castedState == operatorv1alpha1.StateDeleting ||
		castedState == operatorv1alpha1.StateError
}
