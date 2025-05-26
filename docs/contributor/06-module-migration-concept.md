# New Module Submission and Promotion Concept

The existing module metadata is in channel-based ModuleTemplate custom resources (CRs). The new module metadata sources are version-based ModuleTemplates accompanied by ModuleReleaseMeta CRs. This document describes the target module submission process for module development teams. Migrating a Kyma module from the existing module metadata to the new module metadata.

To go directly to the Migration Procedure, see [New Module Submission and Promotion Process: Migration Guide](./07-module-migration-guide.md.md).

> [!Tip]
> Before you start the migration, see the custom resource definitions (CRDs) related to the new module metadata and additional information around the changes.
> - [ModuleTemplate](./resources/03-moduletemplate.md)
> - [ModuleReleaseMeta](./resources/05-modulereleasemeta.md)
> - The Architecture Decision Record backing the migration: [#984](https://github.com/kyma-project/community/issues/984)
> - The PowerPoint presentation with an update given in the 2024-11-26 Kyma Iteration Review meeting: [2024-11-26 Update On Module Metadata](https://sap-my.sharepoint.com/:p:/p/c_schwaegerl/EbvSNmRnr3JEjaLoZ__cI9UB9lu5tt0qaly-f7yQO2Gwbw?e=slfiDf) <!-- markdown-link-check-disable-line -->

## The Target Module Submission and Promotion Process

The following diagram shows the target module submission and promotion process using the Telemetry module. The process consists of the following stages:

1. Submitting a new module version
2. Submitting the channel mapping
3. Promoting ModuleTemplate and ModuleReleaseMeta CRs
4. Deleting a module version

![Target Process](./assets/module-migration.svg)

### 1. Submitting a New Module Version

You submit a new module version by creating a pull request (PR) to the `/kyma/module-manifests` repository. In the PR, you must provide a `module-config.yaml` file under the `/modules/<module-name>/<module-version>` path where `<module-version>` matches the version of the module configured in `module-config.yaml`.

Once the PR is opened, the submission pipeline verifies all the information. For example, the pipeline verifies if the provided `module-config.yaml` is valid (that the FQDN of the module doesn't change), it builds the module using the `modulectl` in `--dry-run` mode, and checks if the version does not exist yet.

Once the PR is merged, the submission pipeline builds and publishes the module using `modulectl` and pushes the generated ModuleTemplate to the `/kyma/kyma-modules` repository. The path of the generated ModuleTemplate is `/<module-name>/moduletemplate-<module-name>-<module-version>.yaml`.

For more information, see the [new submission pipeline](https://github.tools.sap/kyma/test-infra/blob/feature/new-submission-pipeline/ado/new-submission-pipeline-activity.md).

> [!Note]
> The new module version submission process only builds the necessary artifacts and puts them into their respective repositories, namely, the OCI Registry and the `/kyma/kyma-modules` repository. The ModuleTemplate is NOT directly deployed into the KCP landscape.

### 2. Submitting a Channel Mapping

You submit a channel mapping by creating a PR to the `/kyma/module-manifests` repository. In the PR, you must update the `module-releases.yaml` file under the `/modules/<module-name>` path. The `module-releases.yaml` is a simple mapping file defining what version each channel should map to.

Once the PR is opened, the submission pipeline verifies if the mapping is correct. For example, the pipeline verifies that no version downgrade is performed for a channel and that the referenced module version exists.

Once the PR is merged, the submission pipeline generates a ModuleReleaseMeta CR and kustomization, and pushes them to the `/kyma/kyma-modules` repository. The kustomization includes the required ModuleTemplate and the ModuleReleaseMeta CRs.

For more details, see the [new submission pipeline](https://github.tools.sap/kyma/test-infra/blob/feature/new-submission-pipeline/ado/new-submission-pipeline-activity.md).

> [!Note]
>  Because, for example, the `dev` channel is only allowed in the `dev` landscape, and the `experimental` channel is allowed in the `dev` and `stage` landscapes, the ModuleReleaseMeta CR and kustomization are generated landscape-specific. It means there is a separate ModuleReleaseMeta CR and kustomization per landscape.

> [!Note] 
> The kustomization is extended only with ModuleTemplates for the versions referenced in the `module-releases.yaml`. ModuleTemplates for the not-referenced versions are NOT added. Also, versions are not automatically removed from the kustomization, even if not referenced anymore. This needs to be done manually. For more information, see step 4.

> [!Note]
> The new module version and the channel mapping submission processes cannot be combined in one PR. First, the new module version must be submitted. Only then can the channel mapping be updated.

### 3. Promoting ModuleTemplates and ModuleReleaseMeta

ArgoCD detects and applies the changes from Step 2. It deploys only the ModuleTemplates and ModuleReleaseMeta CRs relevant to each landscape.

### 4. Deleting a Module Version

To delete an unused module version, create a PR to the `/kyma/module-manifests` repository. In the PR, delete the module version's `module-config.yaml` file under `/modules/<module-name>/<module-version>`.

Once the PR is opened, the submission pipeline checks if the version is still in use by one or more channels. If so, the PR can't be merged.

Once the PR is merged, the submission pipeline deletes the related ModuleTemplate under `/<module-name>/moduletemplate-<module-name>-<module-version>.yaml` in the `/kyma/kyma-modules` repository. In addition, it removes the reference to the module from the kustomization (*).

ArgoCD picks up these changes to `/kyma/kyma-modules` and undeploys the ModuleTemplate from the landscapes.

For more details, see the [new submission pipeline](https://github.tools.sap/kyma/test-infra/blob/feature/new-submission-pipeline/ado/new-submission-pipeline-activity.md).

> [!Note]
> In the deleting a module version process, only the ModuleTemplate in `/kyma-kyma-modules` gets deleted and undeployed from KCP. The artifacts pushed to the OCI registry remain and cannot be overwritten.

## Related Information

- [New Module Submission and Promotion Process: Migration Guide](./07-module-migration-guide.md.md)
- [Migrating from Kyma CLI to modulectl](https://github.com/kyma-project/modulectl/blob/main/docs/contributor/migration-guide.md)
