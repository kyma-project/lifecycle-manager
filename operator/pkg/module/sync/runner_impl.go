package sync

import (
	"context"
	"fmt"

	manifestV1alpha1 "github.com/kyma-project/module-manager/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
	"github.com/kyma-project/lifecycle-manager/operator/pkg/module/common"
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
	for name := range modules {
		module := modules[name]
		logger := module.Logger(baseLogger)
		manifest := common.NewFromModule(module)
		err := r.getModule(ctx, manifest)

		if errors.IsNotFound(err) {
			logger.Info("module not found, attempting to create it...")
			err := r.createModule(ctx, name, kyma, module)
			if err != nil {
				return false, err
			}
			logger.Info("successfully created module CR")
			return true, nil
		} else if err != nil {
			return false, fmt.Errorf("cannot get module %s: %w", module.GetName(), err)
		}

		module.UpdateStatusAndReferencesFromUnstructured(manifest)
	}

	for name := range modules {
		module := modules[name]
		logger := module.Logger(baseLogger)
		moduleStatus, err := kyma.GetModuleStatusByModuleName(name)
		if err != nil {
			return false, err
		}

		if module.StateMismatchedWithModuleStatus(moduleStatus) {
			if err := r.updateModule(ctx, name, kyma, module); err != nil {
				return false, err
			}
			logger.Info("successfully updated module CR")
			return true, nil
		}
	}

	return false, nil
}

func (r *runnerImpl) getModule(ctx context.Context, module *manifestV1alpha1.Manifest) error {
	return r.Get(ctx, client.ObjectKey{Namespace: module.GetNamespace(), Name: module.GetName()}, module)
}

func (r *runnerImpl) createModule(ctx context.Context, name string, kyma *v1alpha1.Kyma,
	module *common.Module,
) error {
	if err := r.setupModule(module, kyma, name); err != nil {
		return err
	}
	// create resource if not found
	if err := r.Client.Create(ctx, module.Manifest, &client.CreateOptions{}); err != nil {
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

	if err := r.Update(ctx, module.Manifest, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("error updating custom resource of type %s %w", name, err)
	}

	return nil
}

func (r *runnerImpl) setupModule(module *common.Module, kyma *v1alpha1.Kyma, name string) error {
	// set labels
	module.ApplyLabels(kyma, name)

	if module.GetOwnerReferences() == nil {
		// set owner reference
		if err := controllerutil.SetControllerReference(kyma, module.Manifest, r.Scheme()); err != nil {
			return fmt.Errorf("error setting owner reference on component CR of type: %s for resource %s %w",
				name, kyma.Name, err)
		}
	}

	return nil
}

func (r *runnerImpl) SyncModuleStatus(ctx context.Context, kyma *v1alpha1.Kyma, modules common.Modules) bool {
	statusMap := kyma.GetModuleStatusMap()
	statusUpdateRequiredFromUpdate := r.updateModuleStatusFromExistingModules(modules, statusMap, kyma)
	statusUpdateRequiredFromDelete := r.deleteNoLongerExistingModuleStatus(ctx, statusMap, kyma)
	return statusUpdateRequiredFromUpdate || statusUpdateRequiredFromDelete
}

func (r *runnerImpl) updateModuleStatusFromExistingModules(modules common.Modules,
	moduleStatusMap map[string]*v1alpha1.ModuleStatus, kyma *v1alpha1.Kyma,
) bool {
	updateRequired := false
	for name := range modules {
		module := modules[name]
		descriptor, _ := module.Template.Spec.GetUnsafeDescriptor()
		latestModuleStatus := v1alpha1.ModuleStatus{
			ModuleName: module.Name,
			Name:       module.Manifest.GetName(),
			Namespace:  module.Manifest.GetNamespace(),
			Generation: module.Manifest.GetGeneration(),
			TemplateInfo: v1alpha1.TemplateInfo{
				Name:       module.Template.Name,
				Namespace:  module.Template.Namespace,
				Channel:    module.Template.Spec.Channel,
				Generation: module.Template.Generation,
				GroupVersionKind: metav1.GroupVersionKind{
					Group:   manifestV1alpha1.GroupVersionKind.Group,
					Version: manifestV1alpha1.GroupVersionKind.Version,
					Kind:    manifestV1alpha1.GroupVersionKind.Kind,
				},
				Version: descriptor.Version,
			},
			State: stateFromManifest(module.Manifest),
		}
		moduleStatus, exists := moduleStatusMap[module.Name]
		if exists {
			if moduleStatus.State != latestModuleStatus.State {
				updateRequired = true
			}
			*moduleStatus = latestModuleStatus
		} else {
			updateRequired = true
			kyma.Status.ModuleStatus = append(kyma.Status.ModuleStatus, latestModuleStatus)
		}
	}
	return updateRequired
}

func stateFromManifest(obj *manifestV1alpha1.Manifest) v1alpha1.State {
	state := v1alpha1.State(obj.Status.State)
	if state == "" {
		return v1alpha1.StateProcessing
	}
	return state
}

func (r *runnerImpl) deleteNoLongerExistingModuleStatus(ctx context.Context,
	moduleStatusMap map[string]*v1alpha1.ModuleStatus, kyma *v1alpha1.Kyma,
) bool {
	updateRequired := false
	moduleStatusArr := kyma.GetNoLongerExistingModuleStatus()
	if len(moduleStatusArr) == 0 {
		return false
	}
	for i := range moduleStatusArr {
		moduleStatus := moduleStatusArr[i]
		module := manifestV1alpha1.Manifest{}
		module.SetName(moduleStatus.Name)
		module.SetNamespace(moduleStatus.Namespace)
		err := r.getModule(ctx, &module)
		if errors.IsNotFound(err) {
			updateRequired = true
			delete(moduleStatusMap, moduleStatus.ModuleName)
		}
	}
	kyma.Status.ModuleStatus = convertToNewmoduleStatus(moduleStatusMap)
	return updateRequired
}

func convertToNewmoduleStatus(moduleStatusMap map[string]*v1alpha1.ModuleStatus) []v1alpha1.ModuleStatus {
	newModuleStatus := make([]v1alpha1.ModuleStatus, 0)
	for _, moduleStatus := range moduleStatusMap {
		newModuleStatus = append(newModuleStatus, *moduleStatus)
	}
	return newModuleStatus
}
