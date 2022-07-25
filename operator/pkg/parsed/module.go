package parsed

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type (
	Modules map[string]*Module
	Module  struct {
		Name             string
		Template         *v1alpha1.ModuleTemplate
		TemplateOutdated bool
		*unstructured.Unstructured
		Settings unstructured.Unstructured
	}
)

func (m *Module) Channel() v1alpha1.Channel {
	return m.Template.Spec.Channel
}

func (m *Module) Logger(base logr.Logger) logr.Logger {
	return base.WithValues(
		"module", m.Name,
		"channel", m.Channel(),
		"templateGeneration", m.Template.GetGeneration(),
	)
}

func (m *Module) ApplyLabels(
	kyma *v1alpha1.Kyma,
	moduleName string,
) {
	lbls := m.Unstructured.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[v1alpha1.KymaName] = kyma.Name
	lbls[v1alpha1.ProfileLabel] = string(kyma.Spec.Profile)

	lbls[v1alpha1.ModuleName] = moduleName

	templateLabels := m.Template.GetLabels()
	if templateLabels != nil {
		lbls[v1alpha1.ControllerName] = m.Template.GetLabels()[v1alpha1.ControllerName]
	}
	lbls[v1alpha1.ChannelLabel] = string(m.Template.Spec.Channel)

	m.Unstructured.SetLabels(lbls)
}

func (m *Module) CopySettingsToUnstructured() error {
	overrideSpec := m.Settings.Object["spec"]
	if overrideSpec != nil {
		if err := mergo.Merge(m.Unstructured.Object["spec"], overrideSpec); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) StateMismatchedWithCondition(condition *v1alpha1.KymaCondition) bool {
	return condition.TemplateInfo.Generation != m.Template.GetGeneration() ||
		condition.TemplateInfo.Channel != m.Template.Spec.Channel
}

// UpdateStatusFromCluster updates the status of the module based on an interaction with a client that is
// connected to a cluster. It will wrap any error returned from the client, so checking for a k8s error
// can be achievd with Unwrap.
func (m *Module) UpdateStatusFromCluster(ctx context.Context, clnt client.Client) error {
	unstructuredFromServer := unstructured.Unstructured{}
	unstructuredFromServer.SetGroupVersionKind(m.Unstructured.GroupVersionKind())

	if err := clnt.Get(
		ctx,
		client.ObjectKeyFromObject(m.Unstructured),
		&unstructuredFromServer,
	); err != nil {
		return fmt.Errorf("error occurred while fetching module %s: %w", m.GetName(), err)
	}

	m.Unstructured.Object["status"] = unstructuredFromServer.Object["status"]
	m.Unstructured.SetResourceVersion(unstructuredFromServer.GetResourceVersion())
	m.Unstructured.SetGeneration(unstructuredFromServer.GetGeneration())
	return nil
}

func (m *Module) OutdatedCheckWithModuleInfo(moduleInfo *v1alpha1.ModuleInfo) bool {
	if moduleInfo != nil && moduleInfo.Generation != m.Unstructured.GetGeneration() {
		return true
	}
	return false
}
