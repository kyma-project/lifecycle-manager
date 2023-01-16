package catalog

import (
	"context"
	"fmt"

	"github.com/kyma-project/lifecycle-manager/api/v1alpha1"
	remotecontext "github.com/kyma-project/lifecycle-manager/pkg/remote"
	"github.com/kyma-project/module-manager/pkg/types"
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
	moduleTemplatesRuntime := &v1alpha1.ModuleTemplateList{}
	err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime)

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster; then we create it
	if meta.IsNoMatchError(err) {
		if err := c.CreateModuleTemplateCRDInRuntime(ctx, v1alpha1.ModuleTemplateKind.Plural()); err != nil {
			return err
		}
		err = syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime)
	}

	if err != nil {
		return err
	}

	diffApply, diffDelete := c.CalculateDiffs(moduleTemplatesRuntime, moduleTemplatesControlPlane)

	results := make(chan error, len(diffApply)+len(diffDelete))
	for _, diff := range diffApply {
		diff := diff
		go func() {
			diff.SetLastSync()
			if err := syncContext.RuntimeClient.Patch(
				ctx, diff, client.Apply, c.settings.SSAPatchOptions,
			); err != nil {
				results <- fmt.Errorf("could not apply module template diff: %w", err)
			} else {
				results <- nil
			}
		}()
	}
	for _, diff := range diffDelete {
		diff := diff
		go func() {
			if err := syncContext.RuntimeClient.Delete(ctx, diff); err != nil {
				results <- fmt.Errorf("could not delete module template from diff: %w", err)
			} else {
				results <- nil
			}
		}()
	}

	var errs []error
	for i := 0; i < len(diffApply); i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return types.NewMultiError(errs)
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
func (c *RemoteCatalog) CalculateDiffs(
	runtimeList *v1alpha1.ModuleTemplateList, controlPlaneList *v1alpha1.ModuleTemplateList,
) ([]*v1alpha1.ModuleTemplate, []*v1alpha1.ModuleTemplate) {
	// these are various ModuleTemplate references which we will either have to create, update or delete from
	// the remote
	diffToApply := make([]*v1alpha1.ModuleTemplate, 0, len(runtimeList.Items))
	var diffToDelete []*v1alpha1.ModuleTemplate

	// now lets start using two frequency maps to discover diffs
	existingOnRemote := make(map[string]int)
	existingOnControlPlane := make(map[string]int)

	for i := range runtimeList.Items {
		remote := &runtimeList.Items[i]
		existingOnRemote[remote.Namespace+remote.Name] = i
	}

	for i := range controlPlaneList.Items {
		controlPlane := &controlPlaneList.Items[i]
		existingOnControlPlane[controlPlane.Namespace+controlPlane.Name] = i
		// if the controlPlane Template does not exist in the remote, we already know we need to create it
		// in the runtime
		if _, exists := existingOnRemote[controlPlane.Namespace+controlPlane.Name]; !exists {
			c.prepareForSSA(controlPlane)
			diffToApply = append(diffToApply, controlPlane)
		}
	}

	for i := range runtimeList.Items {
		remote := &runtimeList.Items[i]
		controlPlaneIndex, exists := existingOnControlPlane[remote.Namespace+remote.Name]

		// if the remote Template does not exist in the control plane, we already know we need to delete it
		if !exists {
			diffToDelete = append(diffToDelete, remote)
			continue
		}
		c.prepareForSSA(remote)
		(&controlPlaneList.Items[controlPlaneIndex]).Spec.DeepCopyInto(&remote.Spec)
		diffToApply = append(diffToApply, remote)
	}
	return diffToApply, diffToDelete
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
