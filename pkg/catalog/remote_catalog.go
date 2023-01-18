package catalog

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	remotecontext "github.com/kyma-project/lifecycle-manager/pkg/remote"
	"golang.org/x/sync/errgroup"
	v1extensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Settings struct {
	// this namespace flag can be used to override the namespace in which all ModuleTemplates should be applied.
	Namespace       string
	SSAPatchOptions *client.PatchOptions
}

type RemoteCatalog struct {
	settings Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, moduleTemplateList *v1alpha1.ModuleTemplateList) error
	Delete(ctx context.Context) error
}

func NewRemoteCatalogFromKyma(kyma *v1alpha1.Kyma) *RemoteCatalog {
	force := true
	return NewRemoteCatalog(
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: "catalog-sync", Force: &force},
			Namespace:       kyma.Spec.Sync.Namespace,
		},
	)
}

// NewRemoteCatalog uses 2 Clients from a Sync Context to create a Catalog in a remote Cluster.
func NewRemoteCatalog(
	settings Settings,
) *RemoteCatalog {
	return &RemoteCatalog{settings: settings}
}

// CreateOrUpdate first lists all currently available moduleTemplates in the Runtime.
// If there is a NoMatchError, it will attempt to install the CRD
// After the list has been aggregated from the client, it calculates a 2 stage diff
// 1. All ModuleTemplates that either have to be created based on the given Control Plane Templates
// 2. All ModuleTemplates that have to be removed as they were deleted form the Control Plane Templates
// It uses Server-Side-Apply Patches to optimize the turnaround required.
// For more details on when a ModuleTemplate is updated, see CalculateDiffs.
func (c *RemoteCatalog) CreateOrUpdate(
	ctx context.Context,
	moduleTemplatesControlPlane *v1alpha1.ModuleTemplateList,
) error {
	syncContext := remotecontext.SyncContextFromContext(ctx)

	errsApply, applyGroupCtx := errgroup.WithContext(ctx)
	for i := range moduleTemplatesControlPlane.Items {
		template := moduleTemplatesControlPlane.Items[i].DeepCopy()
		c.prepareForSSA(template)
		errsApply.Go(func() error {
			return c.patchDiff(applyGroupCtx, template, syncContext, false)
		})
	}
	if err := errsApply.Wait(); err != nil {
		return err
	}

	moduleTemplatesRuntime := &v1alpha1.ModuleTemplateList{}
	err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime)
	if err != nil {
		return err
	}

	errsDelete, deleteGroupCtx := errgroup.WithContext(ctx)
	for _, diff := range c.diffsToDelete(moduleTemplatesRuntime, moduleTemplatesControlPlane) {
		diff := diff
		errsDelete.Go(func() error {
			return c.patchDiff(deleteGroupCtx, diff, syncContext, true)
		})
	}

	return errsDelete.Wait()
}

func (c *RemoteCatalog) patchDiff(
	ctx context.Context, diff *v1alpha1.ModuleTemplate, syncContext *remotecontext.KymaSynchronizationContext,
	deleteInsteadOfPatch bool,
) error {
	diff.SetLastSync()

	var err error
	if deleteInsteadOfPatch {
		err = syncContext.RuntimeClient.Delete(ctx, diff)
	} else {
		err = syncContext.RuntimeClient.Patch(
			ctx, diff, client.Apply, c.settings.SSAPatchOptions,
		)
	}

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster; then we create it
	if meta.IsNoMatchError(err) {
		if err := c.CreateModuleTemplateCRDInRuntime(ctx, v1alpha1.ModuleTemplateKind.Plural()); err != nil {
			return err
		}
		return c.patchDiff(ctx, diff, syncContext, deleteInsteadOfPatch)
	}

	if err != nil {
		return fmt.Errorf("could not apply module template diff: %w", err)
	}
	return nil
}

// CalculateDiffs takes two ModuleTemplateLists and a given Force Interval and produces 2 Pointer Lists
// The first pointer list references all Templates that would need to be applied to the runtime with SSA
// The second pointer list references all Templates that would need to be deleted from the runtime.
//
// By default, a template is deemed as necessary for apply with SSA to the runtime when
// 1. it does not exist in the controlPlane
// 2. it exists but has a mismatching generation in the control-plane (the control plane spec got updated)
// 3. it exists but has a mismatching generation in the runtime (the runtime spec got updated).
//
// The Diff of the Spec is tracked with two annotations.
// A change in the remote spec advances the last sync gen of the remote
// to the remote generation + 1 during the diff since the expected apply
// would increment the generation.
// This saves an additional API Server call to update the generation in the annotation
// as we already know the spec will change. It also saves us the use of any special status field.
// If for some reason the generation is incremented multiple times in between the current and the next reconciliation
// it can simply jump multiple generations by always basing it on the latest generation of the remote.
func (c *RemoteCatalog) diffsToDelete(
	runtimeList *v1alpha1.ModuleTemplateList, controlPlaneList *v1alpha1.ModuleTemplateList,
) []*v1alpha1.ModuleTemplate {
	kcp := controlPlaneList.Items
	skr := runtimeList.Items
	toDelete := make([]*v1alpha1.ModuleTemplate, 0, len(runtimeList.Items))
	for skrIndex := range skr {
		shouldDeleteFromSKR := true
		for kcpIndex := range kcp {
			if kcp[kcpIndex].Namespace+kcp[kcpIndex].Name == skr[skrIndex].Namespace+skr[skrIndex].Name {
				shouldDeleteFromSKR = false
				break
			}
		}
		if shouldDeleteFromSKR {
			toDelete = append(toDelete, &skr[skrIndex])
		}
	}
	return toDelete
}

func (c *RemoteCatalog) prepareForSSA(moduleTemplate *v1alpha1.ModuleTemplate) {
	moduleTemplate.SetResourceVersion("")
	moduleTemplate.SetUID("")
	moduleTemplate.SetManagedFields([]metav1.ManagedFieldsEntry{})

	if c.settings.Namespace != "" {
		moduleTemplate.SetNamespace(c.settings.Namespace)
	}
}

func (c *RemoteCatalog) Delete(
	ctx context.Context,
) error {
	syncContext := remotecontext.SyncContextFromContext(ctx)
	moduleTemplatesRuntime := &v1alpha1.ModuleTemplateList{Items: []v1alpha1.ModuleTemplate{}}
	err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime)
	if err != nil {
		return err
	}
	for i := range moduleTemplatesRuntime.Items {
		if err := syncContext.RuntimeClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
			!k8serrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (c *RemoteCatalog) CreateModuleTemplateCRDInRuntime(ctx context.Context, plural string) error {
	crd := &v1extensions.CustomResourceDefinition{}
	crdFromRuntime := &v1extensions.CustomResourceDefinition{}

	syncContext := remotecontext.SyncContextFromContext(ctx)

	var err error
	err = syncContext.ControlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crd)

	if err != nil {
		return err
	}

	err = syncContext.RuntimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1alpha1.GroupVersion.Group),
	}, crdFromRuntime)

	if k8serrors.IsNotFound(err) {
		return syncContext.RuntimeClient.Create(ctx, &v1extensions.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: crd.Name, Namespace: crd.Namespace}, Spec: crd.Spec,
		})
	}

	if err != nil {
		return err
	}

	return nil
}
