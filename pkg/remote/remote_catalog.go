package remote

import (
	"context"
	"errors"
	"fmt"
	"github.com/kyma-project/lifecycle-manager/api/shared"
	"k8s.io/apimachinery/pkg/types"

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
	kcpClient         client.Client
	skrContextFactory SkrContextFactory
	settings          Settings
}

type Catalog interface {
	CreateOrUpdate(ctx context.Context, kyma types.NamespacedName, moduleTemplates *v1beta2.ModuleTemplateList) error
	Delete(ctx context.Context) error
}

func NewRemoteCatalogFromKyma(kcpClient client.Client, skrContextFactory SkrContextFactory, remoteSyncNamespace string) *RemoteCatalog {
	force := true
	return NewRemoteCatalog(kcpClient, skrContextFactory,
		Settings{
			SSAPatchOptions: &client.PatchOptions{FieldManager: moduleCatalogSyncFieldManager, Force: &force},
			Namespace:       remoteSyncNamespace,
		},
	)
}

func NewRemoteCatalog(kcpClient client.Client, skrContextFactory SkrContextFactory, settings Settings) *RemoteCatalog {
	return &RemoteCatalog{
		kcpClient:         kcpClient,
		skrContextFactory: skrContextFactory,
		settings:          settings,
	}
}

// CreateOrUpdate first lists all currently available moduleTemplates in the Runtime.
// If there is a NoMatchError, it will attempt to install the CRD but only if there are available crs to copy.
// It will use a 2 stage process:
// 1. All ModuleTemplates that either have to be created based on the given Control Plane Templates
// 2. All ModuleTemplates that have to be removed as they were deleted form the Control Plane Templates
// It uses Server-Side-Apply Patches to optimize the turnaround required.
func (c *RemoteCatalog) CreateOrUpdate(
	ctx context.Context,
	kyma types.NamespacedName,
	kcpModules []v1beta2.ModuleTemplate,
) error {
	skrKymaClient, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SkrContext to update remote catalog: %w", err)
	}
	if err = c.createOrUpdateCatalog(ctx, kyma, kcpModules, skrKymaClient); err != nil {
		return err
	}

	runtimeModules := &v1beta2.ModuleTemplateList{}
	if err := skrKymaClient.Client.List(ctx, runtimeModules); err != nil {
		// it can happen that the ModuleTemplate CRD is not caught during to apply if there are no modules to apply
		// if this is the case and there is no CRD there can never be any module templates to delete
		if meta.IsNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("failed to list module templates from runtime: %w", err)
	}

	return c.deleteDiffCatalog(ctx, kcpModules, runtimeModules.Items, skrKymaClient)
}

var errTemplateCleanup = errors.New("failed to delete obsolete catalog templates")

func (c *RemoteCatalog) deleteDiffCatalog(ctx context.Context,
	kcpModules []v1beta2.ModuleTemplate,
	runtimeModules []v1beta2.ModuleTemplate,
	syncContext *SkrContext,
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
	kyma types.NamespacedName,
	kcpModules []v1beta2.ModuleTemplate,
	syncContext *SkrContext,
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

	// retry if ModuleTemplate CRD is not existing in SKR cluster
	if containsCRDNotFoundError(errs) {
		if err := c.createModuleTemplateCRDInRuntime(ctx, kyma); err != nil {
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
	ctx context.Context, diff *v1beta2.ModuleTemplate, skrContext *SkrContext,
	deleteInsteadOfPatch bool,
) error {
	var err error
	if deleteInsteadOfPatch {
		err = skrContext.Client.Delete(ctx, diff)
	} else {
		err = skrContext.Client.Patch(
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
	kyma types.NamespacedName,
) error {
	skrContext, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SkrContext for deleting RemoteCatalog: %w", err)
	}

	moduleTemplatesRuntime := &v1beta2.ModuleTemplateList{Items: []v1beta2.ModuleTemplate{}}
	if err := skrContext.Client.List(ctx, moduleTemplatesRuntime); err != nil {
		// if there is no CRD or no module template exists,
		// there can never be any module templates to delete
		if util.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to list module templates from skr: %w", err)
	}
	for i := range moduleTemplatesRuntime.Items {
		if isManagedByKcp(moduleTemplatesRuntime.Items[i]) {
			if err := skrContext.Client.Delete(ctx, &moduleTemplatesRuntime.Items[i]); err != nil &&
				!util.IsNotFound(err) {
				return fmt.Errorf("failed to delete module template from skr: %w", err)
			}
		}
	}
	return nil
}

func (c *RemoteCatalog) createModuleTemplateCRDInRuntime(ctx context.Context, kyma types.NamespacedName) error {
	kcpCrd := &apiextensionsv1.CustomResourceDefinition{}
	skrCrd := &apiextensionsv1.CustomResourceDefinition{}
	objKey := client.ObjectKey{Name: fmt.Sprintf("%s.%s", shared.ModuleTemplateKind.Plural(), v1beta2.GroupVersion.Group)}
	err := c.kcpClient.Get(ctx, objKey, kcpCrd)
	if err != nil {
		return fmt.Errorf("failed to get ModuleTemplate CRD from KCP: %w", err)
	}

	skrContext, err := c.skrContextFactory.Get(kyma)
	if err != nil {
		return fmt.Errorf("failed to get SkrContext to create ModuleTemplate CRDs for RemoteCatalog : %w", err)
	}
	err = skrContext.Client.Get(ctx, objKey, skrCrd)

	if util.IsNotFound(err) || !ContainsLatestVersion(skrCrd, v1beta2.GroupVersion.Version) {
		return PatchCRD(ctx, skrContext.Client, kcpCrd)
	}

	if !crdReady(skrCrd) {
		return ErrTemplateCRDNotReady
	}

	if err != nil {
		return fmt.Errorf("failed to get ModuleTemplate CRD from SKR: %w", err)
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
