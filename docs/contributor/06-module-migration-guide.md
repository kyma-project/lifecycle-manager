# Module Migration Guideline

The existing module metadata is in channel-based ModuleTemplate custom resources (CRs). The new module metadata sources are version-based ModuleTemplates accompanied by ModuleReleaseMeta CRs. This document describes the target module submission process for module development teams. migrating a Kyma module from the existing module metadata to the new module metadata.

> [!Tip]
> Before you start the migration, see the custom resource definitions (CRDs) related to the new module metadata and additional information around the changes.
> - [ModuleTemplate](../resources/03-moduletemplate.md)
> - [ModuleReleaseMeta](../resources/05-modulereleasemeta.md)
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

## Migration Path

To ensure a smooth transition, the submission pipeline and KLM currently support **both** the old and new metadata formats. KLM will prefer the new format if both are present. If not, it falls back to the old channel-based metadata.

The migration strategy involves replicating the current state with the **new metadata** while keeping the old metadata as a fallback.


### 1) Submit the Existing Versions with the NEW Approach

First, the module developer re-submits the existing module versions via the new approach. As an example, assume the current module versions are:

- `telemetry-regular` pointing to `1.32.0`
- `telemetry-fast` pointing to `1.34.0`
- `telemetry-experimental` pointing to `1.34.0-experimental`
- `telemetry-dev` pointing to `1.35.0-rc1`

The developer needs to re-submit all versions above via the NEW approach. I.e., they need to submit:

- `/modules/telemetry/1.32.0/module-config.yaml`
- `/modules/telemetry/1.34.0/module-config.yaml`
- `/modules/telemetry/1.34.0-experimental/module-config.yaml`
- `/modules/telemetry/1.35.0-rc1/module-config.yaml`

For necessary changes in the `module-config.yaml` file, see [Migrating from Kyma CLI to `modulectl`](https://github.com/kyma-project/modulectl/blob/main/docs/contributor/migration-guide.md).

Once the versions have been submitted, there are the following ModuleTemplates in `/kyma/kyma-modules`:

- `/telemetry/moduletemplate-telemetry-1.32.0.yaml`
- `/telemetry/moduletemplate-telemetry-1.34.0.yaml`
- `/telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
- `/telemetry/moduletemplate-telemetry-1.35.0-rc1.yaml`

### 2) Submit the Existing Channel Mapping with the NEW Approach

Create a `/module-manifests/modules/<module-name>/module-releases.yaml` that replicates the existing channel mapping. e.g.:

```yaml
channels:
  - channel: regular
    version: 1.32.0
  - channel: fast
    version: 1.34.0
  - channel: experimental
    version: 1.34.0-experimental
  - channel: dev
    version: 1.35.0-rc1
```
Once submitted, this generates landscape-specific ModuleReleaseMeta and updates the kustomizations accordingly in `/kyma/kyma-modules`.

- `/telemetry`
  - `/moduletemplate-telemetry-1.32.0.yaml`
  - `/moduletemplate-telemetry-1.34.0.yaml`
  - `/moduletemplate-telemetry-1.34.0-experimental.yaml`
  - `/moduletemplate-telemetry-1.35.0-rc1.yaml`
  - `/dev`
    - `module-release-meta.yaml` with
      - channel `regular` => `1.32.0`
      - channel `fast` => `1.34.0`
      - channel `experimental` => `1.34.0-experimental`
      - channel `dev` => `1.35.0-rc1`
  - `/stage`
    - `module-release-meta.yaml` with
      - channel `regular` => `1.32.0`
      - channel `fast` => `1.34.0`
      - channel `experimental` => `1.34.0-experimental`
  - `/prod`
    - `module-release-meta.yaml` with
      - channel `regular` => `1.32.0`
      - channel `fast` => `1.34.0`
- `/kustomizations`
  - `/dev`
    - `/kustomization.yaml` with
      - ModuleReleaseMeta `../../telemetry/dev/module-release-meta.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.32.0.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.35.0-rc1.yaml`
      - ... (resources from other modules)
  - `/stage`
    - `/kustomization.yaml` with
      - ModuleReleaseMeta `../../telemetry/stage/module-release-meta.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.32.0.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
      - ... (resources from other modules)
  - `/prod`
    - `/kustomization.yaml` with
      - ModuleReleaseMeta `../../telemetry/stage/module-release-meta.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.32.0.yaml`
      - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0.yaml`
      - ... (resources from other modules)

### 3) Verify in KCP

ArgoCD picks up and deploys the changes from step 2). All landscapes have the same channel-version mapping of the module described in OLD and NEW metadata.

Following the example above, the following resources exist in KCP:

#### New ModuleTemplates
- ModuleTemplate `kyma-system/telemetry-regular` pointing to `1.32.0`
- ModuleTemplate `kyma-system/telemetry-fast` pointing to `1.34.0`
- ModuleTemplate `kyma-system/telemetry-experimental` pointing to `1.34.0-experimental` (only in DEV and STAGE)
- ModuleTemplate `kyma-system/telemetry-dev` pointing to `1.35.0-rc1` (only in DEV)
- ModuleReleaseMeta `kyma-system/telemetry`

#### Old ModuleTemplates
- ModuleTemplate `kyma-system/telemetry-1.32.0`
- ModuleTemplate `kyma-system/telemetry-1.34.0`
- ModuleTemplate `kyma-system/telemetry-1.34.0-experimental` (only in DEV and STAGE)
- ModuleTemplate `kyma-system/telemetry-1.35.0-rc1` (only in DEV)

As the new module metadata takes precedence, the reconciliation of the module already happens based on the new metadata. Since all versions and channel mappings are the same, no update is performed and all modules stay in the same state as before.

The functionality can further be verified by enabling the module in a test SKR which will install it from scratch using the new metadata.

### 4) [FAILURE] Rollback the New Module Metadata

In case a failure happens, the setup can be reverted to the old approach.

To do so, a PR can be opened to `/kyma/kyma-modules` reverting the submission from 2). ArgoCD then undeploys the new module metadata and KLM falls back to the old module metadata.

> Note that after rollback, the old submission pipeline can still be used to submit new versions of the module while working on a fix.

### 5) Submit a Version Upgrade Using the Old Format

To prepare for failure recovery, first submit a version update using the old metadata.

- `/modules/telemetry/regular/module-config.yaml` pointing to `1.34.0` (before `1.32.0`)

Since the new metadata exists, KLM continues to use it. The old metadata is ignored but remains available for rollback.
> This avoids version downgrades if rollback becomes necessary.

### 6) Submit the Updated Channel Mapping with the NEW Approach

After preparing the old metadata to rollback in case of failure, the actual version update using the new metadata can be performed.

Following the example, the module developer submits the following `modules/telemetry/module-releases.yaml` file:

```yaml
channels:
  - channel: regular
    version: 1.34.0 # <= this version is bumped
  - channel: fast
    version: 1.34.0
  - channel: experimental
    version: 1.34.0-experimental
  - channel: dev
    version: 1.35.0-rc1
```

Once the mapping has been submitted, the resources in `/kyma/kyma-modules` equal the ones from step 2), except the `regular` channel pointing to version `1.34.0` in all ModuleReleaseMetas for different landscapes.

### 7) Verify the Module is Updated in KCP

ArgoCD picks up this change and deploys the new ModuleReleaseMeta to the different landscapes. KLM is now picking up the version change and updates all modules using the `regular` channel to version `1.34.0`.

### 8) [FAILURE] Rollback new Metadata

In case a failure happens, the setup can be reverted to the old approach.

To do so, a PR can be opened to `/kyma/kyma-modules` reverting the submissions from 2) and 6). It is important to revert completely removing the entire new metadata from KCP so that KLM falls back to the old module metadata.

> Note that after rollback, the old submission pipeline can still be used to submit new versions of the module while working on a fix.

### 9) Remove the old Metadata

After successful verification, delete all old metadata files related to the module.

### 10) Continue Module Lifecycle with the NEW Approach

Continue to use the new approach to provide new module versions and update the mapping of channels.
