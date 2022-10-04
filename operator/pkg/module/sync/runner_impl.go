package sync

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/common"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/watch"
)

func New(clnt client.Client) Runner { //nolint:ireturn
	return &runnerImpl{clnt}
}

type runnerImpl struct {
	client.Client
}

// Sync implements Runner.Sync.
func (r *runnerImpl) Sync(ctx context.Context, kyma *v1alpha1.Kyma,
	modules common.Modules,
) (bool, error) {
	baseLogger := log.FromContext(ctx).WithName(client.ObjectKey{Name: kyma.Name, Namespace: kyma.Namespace}.String())
	for name, module := range modules {
		logger := module.Logger(baseLogger)

		create := func() (bool, error) {
			logger.Info("module not found, attempting to create it...")
			err := r.createModule(ctx, name, kyma, module)
			if err != nil {
				return false, err
			}
			logger.Info("successfully created module CR")
			return true, nil
		}

		update := func() (bool, error) {
			if err := r.updateModule(ctx, name, kyma, module); err != nil {
				return false, err
			}
			logger.Info("successfully updated module CR")
			return true, nil
		}

		moduleUnstructured := common.NewUnstructuredFromModule(module)
		err := r.getModule(ctx, moduleUnstructured)
		if errors.IsNotFound(err) {
			return create()
		} else if err != nil {
			return false, fmt.Errorf("error occurred while fetching module %s: %w", module.GetName(), err)
		}

		module.UpdateStatusAndReferencesFromUnstructured(moduleUnstructured)

		if module.TemplateOutdated {
			templateInfo, err := kyma.GetTemplateInfoByModuleName(name)
			if err != nil {
				return false, err
			}
			if module.StateMismatchedWithTemplateInfo(templateInfo) {
				return update()
			}
		}
	}

	return false, nil
}

func (r *runnerImpl) getModule(ctx context.Context, module *unstructured.Unstructured) error {
	return r.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
}

func (r *runnerImpl) createModule(ctx context.Context, name string, kyma *v1alpha1.Kyma,
	module *common.Module,
) error {
	if err := r.setupModule(module, kyma, name); err != nil {
		return err
	}
	// create resource if not found
	if err := r.Client.Create(ctx, module.Unstructured, &client.CreateOptions{}); err != nil {
		return fmt.Errorf("error creating custom resource of type %s %w", name, err)
	}

	return nil
}

func (r *runnerImpl) updateModule(ctx context.Context, name string, kyma *v1alpha1.Kyma,
	module *common.Module,
) error {
	if err := r.setupModule(module, kyma, name); err != nil {
		return err
	}

	if err := r.Update(ctx, module.Unstructured, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("error updating custom resource of type %s %w", name, err)
	}

	return nil
}

func (r *runnerImpl) setupModule(module *common.Module, kyma *v1alpha1.Kyma, name string) error {
	// set labels
	module.ApplyLabels(kyma, name)

	if module.GetOwnerReferences() == nil {
		// set owner reference
		if err := controllerutil.SetControllerReference(kyma, module.Unstructured, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				name, kyma.Name, err)
		}
	}

	return nil
}

func (r *runnerImpl) SyncModuleStatus(ctx context.Context, kyma *v1alpha1.Kyma, modules common.Modules) bool {
	statusMap := kyma.GetModuleStatusMap()
	statusUpdateRequiredFromUpdate := r.updatemoduleStatussFromExistingModules(modules, statusMap, kyma)
	statusUpdateRequiredFromDelete := r.deleteNoLongerExistingmoduleStatuss(ctx, statusMap, kyma)
	return statusUpdateRequiredFromUpdate || statusUpdateRequiredFromDelete
}

func (r *runnerImpl) updatemoduleStatussFromExistingModules(modules common.Modules,
	moduleStatusMap map[string]*v1alpha1.ModuleStatus, kyma *v1alpha1.Kyma,
) bool {
	updateRequired := false
	for _, module := range modules {
		latestmoduleStatus := v1alpha1.ModuleStatus{
			ModuleName: module.Name,
			Name:       module.Unstructured.GetName(),
			Namespace:  module.Unstructured.GetNamespace(),
			TemplateInfo: v1alpha1.TemplateInfo{
				Channel:    module.Template.Spec.Channel,
				Generation: module.Template.Generation,
				GroupVersionKind: metav1.GroupVersionKind{
					Group:   module.GroupVersionKind().Group,
					Version: module.GroupVersionKind().Version,
					Kind:    module.GroupVersionKind().Kind,
				},
			},
			State: stateFromUnstructured(module.Unstructured),
		}
		moduleStatus, exists := moduleStatusMap[module.Name]
		if exists {
			if moduleStatus.State != latestmoduleStatus.State {
				updateRequired = true
			}
			*moduleStatus = latestmoduleStatus
		} else {
			updateRequired = true
			kyma.Status.ModuleStatus = append(kyma.Status.ModuleStatus, latestmoduleStatus)
		}
	}
	return updateRequired
}

func stateFromUnstructured(obj *unstructured.Unstructured) v1alpha1.State {
	state, found, err := unstructured.NestedString(obj.Object, watch.Status, watch.State)
	if !found {
		return v1alpha1.StateProcessing
	}
	if err == nil && v1alpha1.IsValidState(state) {
		return v1alpha1.State(state)
	}
	return v1alpha1.StateError
}

func (r *runnerImpl) deleteNoLongerExistingmoduleStatuss(ctx context.Context,
	moduleStatusMap map[string]*v1alpha1.ModuleStatus, kyma *v1alpha1.Kyma,
) bool {
	updateRequired := false
	moduleStatuss := kyma.GetNoLongerExistingmoduleStatuss()
	if len(moduleStatuss) == 0 {
		return false
	}
	for i := range moduleStatuss {
		moduleStatus := moduleStatuss[i]
		module := unstructured.Unstructured{}
		module.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   moduleStatus.TemplateInfo.GroupVersionKind.Group,
			Version: moduleStatus.TemplateInfo.GroupVersionKind.Version,
			Kind:    moduleStatus.TemplateInfo.GroupVersionKind.Kind,
		})
		module.SetName(moduleStatus.Name)
		module.SetNamespace(moduleStatus.Namespace)
		err := r.getModule(ctx, &module)
		if errors.IsNotFound(err) {
			updateRequired = true
			delete(moduleStatusMap, moduleStatus.ModuleName)
		}
	}
	kyma.Status.ModuleStatus = convertToNewmoduleStatuss(moduleStatusMap)
	return updateRequired
}

func convertToNewmoduleStatuss(moduleStatusMap map[string]*v1alpha1.ModuleStatus) []v1alpha1.ModuleStatus {
	newmoduleStatuss := make([]v1alpha1.ModuleStatus, 0)
	for _, moduleStatus := range moduleStatusMap {
		newmoduleStatuss = append(newmoduleStatuss, *moduleStatus)
	}
	return newmoduleStatuss
}
