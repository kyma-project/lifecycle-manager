# New Module Submission and Promotion Process: Migration Guide

To ensure a smooth transition, the submission pipeline, Lifecycle Manager (KLM), and Busola support **both** the old and new metadata formats. If both are present, KLM and Busola use the new format. If no new metadata format is provided, it falls back to the old channel-based metadata.

> [!Note]
> The discriminator for KLM und Busola is the presence of the ModuleReleaseMeta resource. If it is provided, KLM and Busola work exclusively on the new metdata. Old channel-based metadata is ignored. Therefore, make sure to migrate all existing channels to the new metadata.

The migration strategy involves replicating the current state with the new metadata while keeping the old metadata as a fallback.

## Migration Procedure

### Replicating the Current State

As you see in the `targetLandscapes` field in the following steps, first, you perform the migration for the `dev` landscape. Once it is performed and verified for `dev`, you can perform it for `stage` and eventually `prod`.

1. Submit the existing versions using the NEW approach.

   The following example assumes the current module versions are:

   - `telemetry-regular` pointing to `1.32.0`
   - `telemetry-fast` pointing to `1.34.0`
   - `telemetry-experimental` pointing to `1.34.0-experimental`
   - `telemetry-dev` pointing to `1.35.0-rc1`

   You must re-submit all the above versions using the NEW approach. It means you must submit the following:

   - `/modules/telemetry/1.32.0/module-config.yaml`
   - `/modules/telemetry/1.34.0/module-config.yaml`
   - `/modules/telemetry/1.34.0-experimental/module-config.yaml`
   - `/modules/telemetry/1.35.0-rc1/module-config.yaml`


> [!Note]
> For more information on the necessary changes in the `module-config.yaml` file, see [Migrating from Kyma CLI to `modulectl`](https://github.com/kyma-project/modulectl/blob/main/docs/contributor/migration-guide.md#2-module-configuration-module-configyaml-differences).

   After you submit all the versions, the following ModuleTemplate custom resources (CRs) appear in the `/kyma/kyma-modules` repository:

   - `/telemetry/moduletemplate-telemetry-1.32.0.yaml`
   - `/telemetry/moduletemplate-telemetry-1.34.0.yaml`
   - `/telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
   - `/telemetry/moduletemplate-telemetry-1.35.0-rc1.yaml`

2. Submit the existing channel mapping using the **new** approach.

   Create a `/module-manifests/modules/<module-name>/module-releases.yaml` that replicates the existing channel mapping. Target the `dev` landscape only. For example:

   ```yaml
   targetLandscapes:
     - dev
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

   After you submit the channel mapping, the dev-landscape-specific ModuleReleaseMeta CR is created, and the dev-landscape-specific kustomization is updated accordingly in `/kyma/kyma-modules`.

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
   - `/kustomizations`
     - `/dev`
       - `/kustomization.yaml` with
         - ModuleReleaseMeta `../../telemetry/dev/module-release-meta.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.32.0.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.35.0-rc1.yaml`
         - ... (resources from other modules)

3. Verify in `dev` KCP.

   ArgoCD picks up and deploys the changes from the previous step. The `dev` landscape has the same channel-version mapping of the module described in **old** and **new** metadata.

   For the Telemetry module example, the following resources exist in KCP:

   **Old ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-regular` pointing to `1.32.0`
   - ModuleTemplate `kyma-system/telemetry-fast` pointing to `1.34.0`
   - ModuleTemplate `kyma-system/telemetry-experimental` pointing to `1.34.0-experimental`
   - ModuleTemplate `kyma-system/telemetry-dev` pointing to `1.35.0-rc1`

   **New ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-1.32.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0-experimental`
   - ModuleTemplate `kyma-system/telemetry-1.35.0-rc1`
   - ModuleReleaseMeta `kyma-system/telemetry`

   As the new module metadata take precedence, the reconciliation of the module already happens based on the new metadata. And because all versions and channel mappings are the same, no update is performed and all existing installations of the module stay in the same state as before.

   If you want to verify the functionality further, enable the module in a test SKR, which installs it from scratch using the new metadata.

4. In case of failure, roll back the new module metadata.

   If the migration failed, you can revert the setup to the old approach.

   To roll back, open a PR to `/kyma/kyma-modules` reverting the submission to the dev landscape. ArgoCD then undeploys the new module metadata, and KLM falls back to the old module metadata.

> [!Note]
> After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

5. Promote to `stage` landscape.
   
   After you verified on `dev`, promote the new metadata to `stage`.

   To do so, create a PR to `/kyma/kyma-modules` and add the `stage` landscape to the `targetLandscapes` in `/module-manifests/modules/<module-name>/module-releases.yaml`. For example:

   ```yaml
   targetLandscapes:
     - dev
     - stage # <= stage landscape added
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

   After you submit the promotion, the stage-landscape-specific ModuleReleaseMeta CR is created, and the stage-landscape-specific kustomization is updated accordingly in `/kyma/kyma-modules`.

   - `/telemetry`
     - `/moduletemplate-telemetry-1.32.0.yaml`
     - `/moduletemplate-telemetry-1.34.0.yaml`
     - `/moduletemplate-telemetry-1.34.0-experimental.yaml`
     - `/moduletemplate-telemetry-1.35.0-rc1.yaml`
     - `/dev`
       - ... (see step 2.)
     - `/stage`
       - `module-release-meta.yaml` with
         - channel `regular` => `1.32.0`
         - channel `fast` => `1.34.0`
         - channel `experimental` => `1.34.0-experimental`
   - `/kustomizations`
     - `/dev`
       - ... (see step 2.)
     - `/stage`
       - `/kustomization.yaml` with
         - ModuleReleaseMeta `../../telemetry/stage/module-release-meta.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.32.0.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
         - ... (resources from other modules)

> [!Note]
> Channel `dev` is automatically excluded for the `stage` landscape.

6. Verify in `stage` KCP.

   ArgoCD picks up and deploys the changes from the previous step. The `stage` landscape has the same channel-version mapping of the module described in **old** and **new** metadata.

   For the Telemetry module example, the following resources exist in KCP:

   **Old ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-regular` pointing to `1.32.0`
   - ModuleTemplate `kyma-system/telemetry-fast` pointing to `1.34.0`
   - ModuleTemplate `kyma-system/telemetry-experimental` pointing to `1.34.0-experimental`

   **New ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-1.32.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0-experimental`
   - ModuleReleaseMeta `kyma-system/telemetry`

   As the new module metadata takes precedence, the reconciliation of the module already happens based on the new metadata. Because all versions and channel mappings are the same, no update is performed and all installations of the module stay in the same state as before.

   If you want to verify the functionality further, enable the module in a test SKR, which installs it from scratch using the new metadata.

7. In case of failure, roll back the new module metadata.

   If the migration failed, you can revert the setup to the old approach.

   To do so, a PR can be opened to `/kyma/kyma-modules` reverting the submission to the stage landscape. ArgoCD then undeploys the new module metadata and KLM falls back to the old module metadata.

> [!Note]
> After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

8. Promote to `prod` landscape.
   
   After you verified on `stage`, promote the new metadata to `prod`.

   To do so, create a PR to `/kyma/kyma-modules` adding the `prod` landscape to the `targetLandscapes` in `/module-manifests/modules/<module-name>/module-releases.yaml`. For example:

   ```yaml
   targetLandscapes:
     - dev
     - stage
     - prod # <= prod landscape added
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

   After you submit the promotion, the stage-landscape-specific ModuleReleaseMeta CR is created, and the stage-landscape-specific kustomization is updated accordingly in `/kyma/kyma-modules`.

   - `/telemetry`
     - `/moduletemplate-telemetry-1.32.0.yaml`
     - `/moduletemplate-telemetry-1.34.0.yaml`
     - `/moduletemplate-telemetry-1.34.0-experimental.yaml`
     - `/moduletemplate-telemetry-1.35.0-rc1.yaml`
     - `/dev`
       - ... (see step 2.)
     - `/stage`
       - ... (see step 5.)
     - `/prod`
       - `module-release-meta.yaml` with
         - channel `regular` => `1.32.0`
         - channel `fast` => `1.34.0`
   - `/kustomizations`
     - `/dev`
       - ... (see step 2.)
     - `/stage`
       - ... (see step 5.)
     - `/prod`
       - `/kustomization.yaml` with
         - ModuleReleaseMeta `../../telemetry/stage/module-release-meta.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.32.0.yaml`
         - ModuleTemplate `../../telemetry/moduletemplate-telemetry-1.34.0.yaml`
         - ... (resources from other modules)

> [!Note]
> Channels `dev` and `experimental` are automatically excluded for the `prod` landscape.

9. Verify in `prod` KCP.

   ArgoCD picks up and deploys the changes from the previous step. The `prod` landscape has the same channel-version mapping of the module described in **old** and **new** metadata.

   Following the Telemetry module example, the following resources exist in KCP:

   **Old ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-regular` pointing to `1.32.0`
   - ModuleTemplate `kyma-system/telemetry-fast` pointing to `1.34.0`

   **New ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-1.32.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0`
   - ModuleReleaseMeta `kyma-system/telemetry`

   As the new module metadata takes precedence, the reconciliation of the module already happens based on the new metadata. Because all versions and channel mappings are the same, no update is performed and all installations of the module stay in the same state as before.

   The functionality can further be verified by enabling the module in a test SKR which will install it from scratch using the new metadata.

10. In case of failure, roll back the new module metadata.

    If the migration failed, you can revert the setup to the old approach.

    To roll back, open a PR to `/kyma/kyma-modules` reverting the submission to the prod landscape. ArgoCD then undeploys the new module metadata, and KLM falls back to the old module metadata.

> [!Note]
> After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

### Performing a Version Upgrade

Also for version upgrades, first target the `dev` landscape. When the upgrade is performed and verified for `dev`, you can perform it for `stage` and eventually `prod`.

1. Submit a version upgrade using the old format.

   To avoid version downgrades, prepare for failure recovery and submit a version update using the old metadata.

   - `/modules/telemetry/regular/module-config.yaml` pointing to `1.34.0` (before `1.32.0`)

   Because the new metadata exists, KLM continues to use it. The old metadata is ignored but remains available for rollback.

2. Submit the updated channel mapping with the **new** approach.

   After preparing the old metadata to rollback in case of failure, you can continue with the actual version update using the new metadata. Target the `dev` landscpae only.

   Following the example, submit the `modules/telemetry/module-releases.yaml` file:

   ```yaml
   targetLandscapes:
     - dev # <= only targeting dev again
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

   After you submit the mapping, the resources in `/kyma/kyma-modules` are the same as in the [Replicating the Current State](#replicating-the-current-state) scenario , except that the `regular` channel points to version `1.34.0` in the `dev` ModuleReleaseMeta CR. The ModuleReleaseMeta CRs for the other landscapes should remain untouched.

3. Verify if the module is updated in KCP.

   ArgoCD picks up this change and deploys the new ModuleReleaseMeta to the `dev` landscape. KLM is now picking up the version change and updating all modules using the `regular` channel to version `1.34.0`.

4. In case of failure, roll back the new metadata.

   If the version upgrade failed, you can revert the setup to the old approach.

   To roll back, open a PR to `/kyma/kyma-modules` reverting the submissions from **both** [Replicating the Current State](#replicating-the-current-state) and [Performing a Version Upgrade]. 
   It is important to revert completely removing the entire new metadata from KCP. Otherwise, KLM cannot fall back to the old module metadata.

> [!Note]
> After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

5. Promote to `stage` landscape

   After you verified on `dev`, promote the new metadata to `stage`.

   To do so, create a PR to `/kyma/kyma-modules` adding the `stage` landscape to the `targetLandscapes` in `/module-manifests/modules/<module-name>/module-releases.yaml`. For example:

   ```yaml
   targetLandscapes:
     - dev
     - stage # <= stage landscape added
   channels:
     - channel: regular
       version: 1.34.0
     - channel: fast
       version: 1.34.0
     - channel: experimental
       version: 1.34.0-experimental
     - channel: dev
       version: 1.35.0-rc1
   ```

6. Verify if the module is updated in KCP.

   ArgoCD picks up this change and deploys the new ModuleReleaseMeta to the `stage` landscape. KLM is now picking up the version change and updating all modules using the `regular` channel to version `1.34.0`.

7.  In case of failure, roll back the new metadata.

   If the version upgrade failed, you can revert the setup to the old approach.

   To roll back, open a PR to `/kyma/kyma-modules` reverting the submissions from **both** [Replicating the Current State](#replicating-the-current-state) and [Performing a Version Upgrade].
   It is important to revert completely removing the entire new metadata from KCP. Otherwise, KLM cannot fall back to the old module metadata.

> [!Note]
> After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

8. Promote to `prod` landscape.

   After you verified on `stage`, promote the new metadata to `prod`.

   To do so, create a PR to `/kyma/kyma-modules` adding the `prod` landscape to the `targetLandscapes` in `/module-manifests/modules/<module-name>/module-releases.yaml`. For example:

   ```yaml
   targetLandscapes:
     - dev
     - stage
     - prod # <= prod landscape added
   channels:
     - channel: regular
       version: 1.34.0
     - channel: fast
       version: 1.34.0
     - channel: experimental
       version: 1.34.0-experimental
     - channel: dev
       version: 1.35.0-rc1
   ```

9. Verify if the module is updated in KCP.

   ArgoCD picks up this change and deploys the new ModuleReleaseMeta to the `prod` landscape. KLM is now picking up the version change and updating all modules using the `regular` channel to version `1.34.0`.

10. In case of failure, roll back the new metadata.

   In case of failure, you can revert the setup to the old approach.

   To roll back, open a PR to `/kyma/kyma-modules` reverting the submissions from **both** [Replicating the Current State](#replicating-the-current-state) and [Performing a Version Upgrade].
   It is important to revert completely removing the entire new metadata from KCP. Otherwise, KLM cannot fall back to the old module metadata.

> [!Note]
> After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

### Cleaning Up

1. Delete all old metadata files related to the module.

   To do so, submit a PR deleting the old channel-based ModuleTemplates of the module.

2. Verify on KCP.

   ArgoCD picks up this change and removes the ModuleTemplates from all KCP landscapes. Verify that those are gone.

### Continuing

1. Continue using the **new** approach to provide new module versions and update the mapping of channels.

   Submit new module versions and release them with the [new processes](./06-module-migration-concept.md).

2. Delete unused versions.

   After all installations of a module version have been updated to a newer version, remove the unused ModuleTemplates following the [Deleting a Module Version](./06-module-migration-concept.md#4-deleting-a-module-version) process.
