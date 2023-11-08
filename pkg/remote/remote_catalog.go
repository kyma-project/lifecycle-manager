package remote

import (
	"context"
	"errors"
	"fmt"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apimetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/lifecycle-manager/api/v1beta2"
	"github.com/kyma-project/lifecycle-manager/pkg/util"
)

var ErrTemplateCRDNotReady = errors.New("module template crd for catalog sync is not ready")

const moduleCatalogSyncFieldManager = "catalog-sync"

type Settings struct {
	// this namespace flag can be used to override the namespace in which all ModuleTemplates should be applied.
	Namespace       string
	SSAPatchOptions *client.PatchOptions
}

type RemoteCatalog struct {
	settings Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, moduleTemplateList *v1beta2.ModuleTemplateList) error
	Delete(ctx context.Context) error
}

func NewRemoteCatalogFromKyma(remoteSyncNamespace string) *RemoteCatalog {
	force := true
	return NewRemoteCatalog(
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: moduleCatalogSyncFieldManager, Force: &force},
			Namespace:       remoteSyncNamespace,
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
// If there is a NoMatchError, it will attempt to install the CRD but only if there are available crs to copy.
// It will use a 2 stage process:
// 1. All ModuleTemplates that either have to be created based on the given Control Plane Templates
// 2. All ModuleTemplates that have to be removed as they were deleted form the Control Plane Templates
// It uses Server-Side-Apply Patches to optimize the turnaround required.
func (c *RemoteCatalog) CreateOrUpdate(
	ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
) error {
	syncContext, err := SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}
	if err := c.createOrUpdateCatalog(ctx, kcpModules, syncContext); err != nil {
		return err
	}

	runtimeModules := &v1beta2.ModuleTemplateList{}
	if err := syncContext.RuntimeClient.List(ctx, runtimeModules); err != nil {
		// it can happen that the ModuleTemplate CRD is not caught during to apply if there are no modules to apply
		// if this is the case and there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed to list module templates from runtime: %w", err)
	}

	return c.deleteDiffCatalog(ctx, kcpModules, runtimeModules.Items, syncContext)
}

var errTemplateCleanup = errors.New("failed to delete obsolete catalog templates")

func (c *RemoteCatalog) deleteDiffCatalog(ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
	runtimeModules []v1beta2.ModuleTemplate,
	syncContext *KymaSynchronizationContext,
) error {
	diffsToDelete := c.diffsToDelete(runtimeModules, kcpModules)
	channelLength := len(diffsToDelete)
	results := make(chan error, channelLength)
	for _, diff := range diffsToDelete {
		diff := diff
		go func() {
			results <- c.patchDiff(ctx, diff, syncContext, true)
		}()
	}
	var errs []error
	for i := 0; i < channelLength; i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errTemplateCleanup)
		return errors.Join(errs...)
	}
	return nil
}

var errCatTemplatesApply = errors.New("could not apply catalog templates")

func (c *RemoteCatalog) createOrUpdateCatalog(ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
	syncContext *KymaSynchronizationContext,
) error {
	channelLength := len(kcpModules)
	results := make(chan error, channelLength)
	for kcpIndex := range kcpModules {
		kcpIndex := kcpIndex
		go func() {
			c.prepareForSSA(&kcpModules[kcpIndex])
			results <- c.patchDiff(ctx, &kcpModules[kcpIndex], syncContext, false)
		}()
	}
	var errs []error
	for i := 0; i < channelLength; i++ {
		if err := <-results; err != nil {
			errs = append(errs, err)
		}
	}

	// it can happen that the ModuleTemplate CRD is not existing in the Remote Cluster when we apply it and retry
	if containsCRDNotFoundError(errs) {
		if err := c.CreateModuleTemplateCRDInRuntime(ctx, v1beta2.ModuleTemplateKind.Plural()); err != nil {
			return err
		}
	}

	if len(errs) != 0 {
		errs = append(errs, errCatTemplatesApply)
		return errors.Join(errs...)
	}
	return nil
}

func containsCRDNotFoundError(errs []error) bool {
	for _, err := range errs {
		unwrappedError := errors.Unwrap(err)
		if meta.IsNoMatchError(unwrappedError) || CRDNotFoundErr(unwrappedError) {
			return true
		}
	}
	return false
}

func (c *RemoteCatalog) patchDiff(
	ctx context.Context, diff *v1beta2.ModuleTemplate, syncContext *KymaSynchronizationContext,
	deleteInsteadOfPatch bool,
) error {
	var err error
	if deleteInsteadOfPatch {
		err = syncContext.RuntimeClient.Delete(ctx, diff)
	} else {
		err = syncContext.RuntimeClient.Patch(
			ctx, diff, client.Apply, c.settings.SSAPatchOptions,
		)
	}

	if err != nil {
		return fmt.Errorf("could not apply module template diff: %w", err)
	}
	return nil
}

// diffsToDelete takes 2 v1beta2.ModuleTemplateList to then calculate any diffs.
// Diffs are defined as any v1beta2.ModuleTemplate that is available in the skrList but not in the kcpList.
func (c *RemoteCatalog) diffsToDelete(
	skrList []v1beta2.ModuleTemplate, kcpList []v1beta2.ModuleTemplate,
) []*v1beta2.ModuleTemplate {
	toDelete := make([]*v1beta2.ModuleTemplate, 0, len(skrList))
	presentInKCP := make(map[string]struct{}, len(kcpList))
	for i := range kcpList {
		presentInKCP[kcpList[i].Namespace+kcpList[i].Name] = struct{}{}
	}
	for i := range skrList {
		if _, inKCP := presentInKCP[skrList[i].Namespace+skrList[i].Name]; !inKCP && isManagedByKcp(skrList[i]) {
			toDelete = append(toDelete, &skrList[i])
		}
	}
	return toDelete
}

func isManagedByKcp(skrTemplate v1beta2.ModuleTemplate) bool {
	for _, managedFieldEntry := range skrTemplate.ObjectMeta.ManagedFields {
		if managedFieldEntry.Manager == moduleCatalogSyncFieldManager {
			return true
		}
	}
	return false
}

func (c *RemoteCatalog) prepareForSSA(moduleTemplate *v1beta2.ModuleTemplate) {
	moduleTemplate.SetResourceVersion("")
	moduleTemplate.SetUID("")
	moduleTemplate.SetManagedFields([]apimetav1.ManagedFieldsEntry{})

	if c.settings.Namespace != "" {
		moduleTemplate.SetNamespace(c.settings.Namespace)
	}
}

func (c *RemoteCatalog) Delete(
	ctx context.Context,
) error {
	syncContext, err := SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}

	moduleTemplatesRuntime := &v1beta2.ModuleTemplateList{Items: []v1beta2.ModuleTemplate{}}
	if err := syncContext.RuntimeClient.List(ctx, moduleTemplatesRuntime); err != nil {
		// if there is no CRD or no module template exists,
		// there can never be any module templates to delete
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to list module templates on runtime: %w", err)
	}
	for i := range moduleTemplatesRuntime.Items {
		if isManagedByKcp(moduleTemplatesRuntime.Items[i]) {
			if err := syncContext.RuntimeClient.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
				!util.IsNotFound(err) {
				return fmt.Errorf("failed to delete module template from runtime: %w", err)
			}
		}
	}
	return nil
}

func (c *RemoteCatalog) CreateModuleTemplateCRDInRuntime(ctx context.Context, plural string) error {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	crdFromRuntime := &apiextensionsv1.CustomResourceDefinition{}

	var err error

	syncContext, err := SyncContextFromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get syncContext: %w", err)
	}

	err = syncContext.ControlPlaneClient.Get(ctx, client.ObjectKey{
		// this object name is derived from the plural and is the default kustomize value for crd namings, if the CRD
		// name changes, this also has to be adjusted here. We can think of making this configurable later
		Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
	}, crd)

	if err != nil {
		return fmt.Errorf("failed to get module template CRD from kcp: %w", err)
	}

	err = syncContext.RuntimeClient.Get(ctx, client.ObjectKey{
		Name: fmt.Sprintf("%s.%s", plural, v1beta2.GroupVersion.Group),
	}, crdFromRuntime)

	if util.IsNotFound(err) || !ContainsLatestVersion(crdFromRuntime, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, syncContext.RuntimeClient, crd)
	}

	if !crdReady(crdFromRuntime) {
		return ErrTemplateCRDNotReady
	}

	if err != nil {
		return fmt.Errorf("failed to get module template CRD from runtime: %w", err)
	}

	return nil
}

func crdReady(crd *apiextensionsv1.CustomResourceDefinition) bool {
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextensionsv1.Established &&
			cond.Status == apiextensionsv1.ConditionTrue {
			return true
		}

		if cond.Type == apiextensionsv1.NamesAccepted &&
			cond.Status == apiextensionsv1.ConditionFalse {
			// This indicates a naming conflict, but it's probably not the
			// job of this function to fail because of that. Instead,
			// we treat it as a success, since the process should be able to
			// continue.
			return true
		}
	}
	return false
}
