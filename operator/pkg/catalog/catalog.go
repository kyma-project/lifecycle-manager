package catalog

import (
	"context"
	"errors"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/kyma-operator/operator/api/v1alpha1"
)

type Settings struct {
	Name      string
	Namespace string
}

type entry struct {
	Name               string                     `json:"name"`
	Defaults           *unstructured.Unstructured `json:"defaults"`
	Channel            v1alpha1.Channel           `json:"channel"`
	Target             v1alpha1.Target            `json:"target"`
	Version            string                     `json:"version"`
	TemplateGeneration int64                      `json:"templateGeneration"`
}

type catalogImpl struct {
	clnt     client.Client
	settings Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, moduleTemplates []v1alpha1.ModuleTemplate) error
	Delete(ctx context.Context) error
	Client() client.Client
	Settings() Settings
}

func New(
	clnt client.Client,
	settings Settings,
) *catalogImpl {
	return &catalogImpl{clnt: clnt, settings: settings}
}

func (c *catalogImpl) CreateOrUpdate(
	ctx context.Context,
	moduleTemplates []v1alpha1.ModuleTemplate,
) error {
	clnt := c.clnt
	settings := c.settings

	catalog := &v1.ConfigMap{}
	catalog.SetName(settings.Name)
	catalog.SetNamespace(settings.Namespace)

	create := false
	err := clnt.Get(ctx, client.ObjectKeyFromObject(catalog), catalog)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	create = k8serrors.IsNotFound(err)

	if catalog.Data == nil {
		catalog.Data = make(map[string]string)
	}

	templatesNeedUpdate := false
	for i := range moduleTemplates {
		moduleTemplate := &moduleTemplates[i]
		moduleName, found := moduleTemplate.GetLabels()[v1alpha1.ModuleName]
		if !found {
			return errors.New(v1alpha1.ModuleName + " does not exist on moduletemplate")
		}
		var yml []byte
		var err error

		yml, err = yaml.Marshal(&entry{
			Name:               moduleName,
			Defaults:           &moduleTemplate.Spec.Data,
			Channel:            moduleTemplate.Spec.Channel,
			Target:             moduleTemplate.Spec.Target,
			Version:            moduleTemplate.GetLabels()[v1alpha1.ModuleVersion],
			TemplateGeneration: moduleTemplate.GetGeneration(),
		})

		if !templatesNeedUpdate && c.doesModuleTemplateNeedUpdateInCatalog(catalog, moduleTemplate) {
			templatesNeedUpdate = true
		}

		if err != nil {
			return err
		}

		catalog.Data[moduleName] = string(yml)
	}

	if create {
		return clnt.Create(ctx, catalog)
	}
	if templatesNeedUpdate {
		return clnt.Update(ctx, catalog)
	}
	return nil
}

func (c *catalogImpl) doesModuleTemplateNeedUpdateInCatalog(
	catalog *v1.ConfigMap,
	template *v1alpha1.ModuleTemplate,
) bool {
	moduleName := template.GetLabels()[v1alpha1.ModuleName]
	if catalog.Data[moduleName] != "" {
		oldCatalogEntry := &entry{}
		if err := yaml.Unmarshal([]byte(catalog.Data[moduleName]), oldCatalogEntry); err != nil ||
			oldCatalogEntry.TemplateGeneration != template.Generation {
			return true
		}
	}
	return true
}

func (c *catalogImpl) Delete(
	ctx context.Context,
) error {
	catalog := &v1.ConfigMap{}
	catalog.SetName(c.settings.Name)
	catalog.SetNamespace(c.settings.Namespace)
	return c.clnt.Delete(ctx, catalog)
}

func (c *catalogImpl) Client() client.Client {
	return c.clnt
}

func (c *catalogImpl) Settings() Settings {
	return c.settings
}
