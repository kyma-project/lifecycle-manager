package catalog

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/operator/api/v1alpha1"
)

type Settings struct{}

type Impl struct {
	runtimeClient      client.Client
	controlPlaneClient client.Client
	settings           Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, moduleTemplateList *v1alpha1.ModuleTemplateList) error
	Delete(ctx context.Context) error
	Client() client.Client
	Settings() Settings
}

func New(
	runtimeClient client.Client,
	controlPlaneClient client.Client,
	settings Settings,
) *Impl {
	return &Impl{runtimeClient: runtimeClient, controlPlaneClient: controlPlaneClient, settings: settings}
}

func (c *Impl) CreateOrUpdate(
	ctx context.Context,
	moduleTemplatesControlPlane *v1alpha1.ModuleTemplateList,
) error {
	moduleTemplatesRuntime := &v1alpha1.ModuleTemplateList{}
	err := c.runtimeClient.List(ctx, moduleTemplatesRuntime)

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster, then we create it
	if meta.IsNoMatchError(err) {
		if err := c.CreateCRD(ctx, v1alpha1.ModuleTemplateKind.Plural()); err != nil {
			return err
		}
		err = c.runtimeClient.List(ctx, moduleTemplatesRuntime)
	}

	if err != nil {
		return err
	}

	diffToCreate, diffToUpdate, diffToDelete := calculateDiffs(moduleTemplatesRuntime, moduleTemplatesControlPlane)

	for _, diff := range diffToCreate {
		if diff.Annotations == nil {
			diff.Annotations = map[string]string{}
		}
		diff.Annotations[v1alpha1.LastSync] = time.Now().String()
		diff.Annotations[v1alpha1.LastSyncGeneration] = strconv.Itoa(int(diff.GetGeneration()))
		if err := c.runtimeClient.Create(ctx, diff); err != nil {
			return fmt.Errorf("could not create module template from discovered diff: %w", err)
		}
	}

	for _, diff := range diffToUpdate {
		if diff.Annotations == nil {
			diff.Annotations = map[string]string{}
		}
		diff.Annotations[v1alpha1.LastSync] = time.Now().String()
		diff.Annotations[v1alpha1.LastSyncGeneration] = strconv.Itoa(int(diff.GetGeneration()))
		if err := c.runtimeClient.Update(ctx, diff); err != nil {
			return fmt.Errorf("could not create module template from discovered diff: %w", err)
		}
	}

	for _, diff := range diffToDelete {
		if err := c.runtimeClient.Delete(ctx, diff); err != nil {
			return fmt.Errorf("could not delete module template from discovered diff: %w", err)
		}
	}
	return nil
}

func calculateDiffs(
	moduleTemplatesRuntime *v1alpha1.ModuleTemplateList, moduleTemplatesControlPlane *v1alpha1.ModuleTemplateList,
) ([]*v1alpha1.ModuleTemplate, []*v1alpha1.ModuleTemplate, []*v1alpha1.ModuleTemplate) {
	// these are various ModuleTemplate references which we will either have to create, update or delete from
	// the remote
	var diffToCreate []*v1alpha1.ModuleTemplate
	var diffToUpdate []*v1alpha1.ModuleTemplate
	var diffToDelete []*v1alpha1.ModuleTemplate

	// now lets start using two frequency maps to discover diffs
	existingOnRemote := make(map[string]int)
	existingOnControlPlane := make(map[string]int)

	for i := range moduleTemplatesRuntime.Items {
		remote := &moduleTemplatesRuntime.Items[i]
		existingOnRemote[remote.Namespace+remote.Name] = i
	}

	for i := range moduleTemplatesControlPlane.Items {
		controlPlane := &moduleTemplatesControlPlane.Items[i]
		existingOnControlPlane[controlPlane.Namespace+controlPlane.Name] = i

		// if the controlPlane Template does not exist in the remote, we already know we need to create it
		// in the runtime
		if _, exists := existingOnRemote[controlPlane.Namespace+controlPlane.Name]; !exists {
			controlPlane.SetResourceVersion("")
			controlPlane.SetOwnerReferences(nil)
			diffToCreate = append(diffToCreate, controlPlane)
		}
	}

	for i := range moduleTemplatesRuntime.Items {
		remote := &moduleTemplatesRuntime.Items[i]
		controlPlaneIndex, exists := existingOnControlPlane[remote.Namespace+remote.Name]

		// if the remote Template does not exist in the control plane, we already know we need to delete it
		if !exists {
			diffToDelete = append(diffToDelete, remote)
			continue
		}

		// if there is a template in controlPlane and remote, but the generation is outdated, we need to
		// update it
		remoteSyncedGen, _ := strconv.Atoi(remote.Annotations[v1alpha1.LastSyncGeneration])
		if int64(remoteSyncedGen) != (&moduleTemplatesControlPlane.Items[controlPlaneIndex]).GetGeneration() {
			diffToUpdate = append(diffToUpdate, remote)
		}
	}
	return diffToCreate, diffToUpdate, diffToDelete
}

func (c *Impl) Delete(
	ctx context.Context,
) error {
	var moduleTemplatesRuntime *v1alpha1.ModuleTemplateList
	err := c.runtimeClient.List(ctx, moduleTemplatesRuntime)
	if err != nil {
		return err
	}
	for i := range moduleTemplatesRuntime.Items {
		if err := c.runtimeClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Impl) Client() client.Client {
	return c.runtimeClient
}

func (c *Impl) Settings() Settings {
	return c.settings
}

func (c *Impl) CreateCRD(ctx context.Context, plural string) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}
	var err error
	err = c.controlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = c.runtimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crdFromRuntime)

	if k8serrors.IsNotFound(err) {
		return c.runtimeClient.Create(ctx, &v1extensions.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
		})
	}

	if err != nil {
		return err
	}

	// crd.SetResourceVersion(crdFromRuntime.GetResourceVersion())
	// return c.runtimeClient.Update(ctx, &v1extensions.CustomResourceDefinition{
	// 	ObjectMeta: v1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
	// })
	return nil
}
