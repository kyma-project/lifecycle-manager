# Module Migration Guideline

This guide provides detailed instructions for how to migrate a Kyma module from existing (old) module metadata, especially channel-based ModuleTemplates, to the new module metadata, i.e. version-based ModuleTemplates accompanied by ModuleReleaseMeta.

It is highly recommended that module teams familiarize themselves with the new module metadata before starting the migration:
- The ADR backing the migration is [#984](https://github.com/kyma-project/community/issues/984)
- An Update on Module Metadata has also been given in the 2024-11-26 Iteration Review

## Target Process

The target process is shown in the figure below. As an example, the `telemetry` module is used. The promotion of module versions and their assignments to channels is managed via three individual processes. In addition, there is a fourth process to delete a module version.

![Target Process](./assets/module-migration.svg)

### 1) Submitting a new module version

First, the module developer submits a new module version via a PR to the `/kyma/module-manifests` repository. The submission must provide a `module-config.yaml` file under the path `/modules/<module-name>/<module-version>` where `<module-version>` matches the version of the module configured in `module-config.yaml`. For the detailed format of the `module-config.yaml`, see below.

Once the PR is opened, the submission pipeline verifies that the info is correct. E.g., it verifies that the provided `module-config.yaml` is valid (e.g., it verifies that the FQDN of the module doesn't change), it builds the module via `modulectl` in `--dry-run` mode, and it verifies that the version does not exist yet.

Once the PR is merged, the submission pipeline builds and publishes the module via `modulectl` and pushes the generated ModuleTemplate to the `/kyma/kyma-modules` repository. The path of the generated ModuleTemplate is `/<module-name>/moduletemplate-<module-name>-<module-version>.yaml`.

For more details, see [new submission pipeline](https://github.tools.sap/kyma/test-infra/blob/feature/new-submission-pipeline/ado/new-submission-pipeline-activity.md).

> Note that this process only builds the necessary artifacts and puts them into their repositories, i.e. the OCI Registry and the `/kyma/kyma-modules` repository. The ModuleTemplate is NOT directly deployed into the KCP landscape.

### 2) Submitting a channel mapping

Second, the module developer submits a channel mapping via a PR to the `/kyma/module-manifests` repository. The submission must update the `module-releases.yaml` file under the path `/modules/<module-name>`. The `module-releases.yaml` is a simple mapping file to define what version each channel should map to. For the detailed format of the `module-releases.yaml`, see below.

Once the PR is opened, the submission pipeline verifies that the mapping is correct. E.g., it verifies that no version downgrade is performed for a channel and it verifies that the referenced module version exists.

Once the PR is merged, the submission pipeline generates the resulting ModuleReleaseMeta and kustomization and pushes them to the `/kyma/kyma-modules` repository. The kustomization includes the required ModuleTemplates and the ModuleReleaseMeta.

For more details, see [new submission pipeline](https://github.tools.sap/kyma/test-infra/blob/feature/new-submission-pipeline/ado/new-submission-pipeline-activity.md).

> Note that the ModuleReleaseMeta and kustomization are generated landscape specific. I.e., there is a separate ModuleReleaseMeta and kustomization per landscape. Reason behind this is that the `dev` channel is only allowed in `dev` landscape, and `experimental` channel is only allowed in `dev` and `stage` landscapes.

> Note that the kustomization is extended with ModuleTemplates for the versions referenced in the `module-releases.yaml` only. ModuleTemplates for versions not referenced are NOT added. Also, versions are not automatically removed from the kustomization, even if not referenced anymore. This needs to be done manually, see step 4).

> Note that this process and the previous one cannot be combined in one PR. First, the new module version must be submitted via 1), only then the channel mapping can be updated via 2).

### 3) Promoting ModuleTemplates and ModuleReleaseMeta

ArgoCD detects and applies the changes from Step 2. It deploys only the `ModuleTemplates` and `ModuleReleaseMeta` relevant to each landscape.

### 4) Deleting a module version

To delete a unused module version, a PR to the `/kyma/module-manifests` repository is opened deleting the module versions' `module-config.yaml` file under `/modules/<module-name>/<module-version>`.

Once the PR is opened, the submission pipeline checks if the version is still in use by one or more channels. If so, the PR can't be merged.

Once the PR is merged, the submission pipeline deletes the related ModuleTemplate `/<module-name>/moduletemplate-<module-name>-<module-version>.yaml` in the `/kyma/kyma-modules` repository. In addition, it removes the reference to the module from the kustomization (*).

ArgoCD picks these changes to `/kyma/kyma-modules` up and undeploys the ModuleTemplate from the landscapes.

For more details, see [New submission pipeline](https://github.tools.sap/kyma/test-infra/blob/feature/new-submission-pipeline/ado/new-submission-pipeline-activity.md).

> Note that this only deletes the ModuleTemplate in `/kyma-kyma-modules` and undeploys it from KCP. The artifacts pushed to the OCI registry remain and cannot be overwritten.

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
