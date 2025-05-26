# New Module Submission and Promotion Process: Migration Guide

To ensure a smooth transition, the submission pipeline, and Lifecycle Manager support **both** the old and new metadata formats. If both are present, Lifecycle Manager prefers the new format. If no new metadata format is provided, it falls back to the old channel-based metadata.

The migration strategy involves replicating the current state with the NEW metadata while keeping the old metadata as a fallback.

## Migration Procedure

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

   For more information on the necessary changes in the `module-config.yaml` file, see [Migrating from Kyma CLI to `modulectl`](https://github.com/kyma-project/modulectl/blob/main/docs/contributor/migration-guide.md).

   Once you submit all the versions, the following ModuleTemplate custom resources (CRs) appear in the `/kyma/kyma-modules` repository:

   - `/telemetry/moduletemplate-telemetry-1.32.0.yaml`
   - `/telemetry/moduletemplate-telemetry-1.34.0.yaml`
   - `/telemetry/moduletemplate-telemetry-1.34.0-experimental.yaml`
   - `/telemetry/moduletemplate-telemetry-1.35.0-rc1.yaml`

2. Submit the existing channel mapping using the NEW approach.

   Create a `/module-manifests/modules/<module-name>/module-releases.yaml` that replicates the existing channel mapping. For example:

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

   Once you submit the channel mapping, landscape-specific ModuleReleaseMeta CRs are created and kustomizations are updated accordingly in `/kyma/kyma-modules`.

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

3. Verify in KCP.

   ArgoCD picks up and deploys the changes from step 2. All landscapes have the same channel-version mapping of the module described in OLD and NEW metadata.

   Following the Telemetry module example, the following resources exist in KCP:

   **New ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-regular` pointing to `1.32.0`
   - ModuleTemplate `kyma-system/telemetry-fast` pointing to `1.34.0`
   - ModuleTemplate `kyma-system/telemetry-experimental` pointing to `1.34.0-experimental` (only in DEV and STAGE)
   - ModuleTemplate `kyma-system/telemetry-dev` pointing to `1.35.0-rc1` (only in DEV)
   - ModuleReleaseMeta `kyma-system/telemetry`

   **Old ModuleTemplates**
   - ModuleTemplate `kyma-system/telemetry-1.32.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0`
   - ModuleTemplate `kyma-system/telemetry-1.34.0-experimental` (only in DEV and STAGE)
   - ModuleTemplate `kyma-system/telemetry-1.35.0-rc1` (only in DEV)

   As the new module metadata takes precedence, the reconciliation of the module already happens based on the new metadata. Since all versions and channel mappings are the same, no update is performed and all modules stay in the same state as before.

   The functionality can further be verified by enabling the module in a test SKR which will install it from scratch using the new metadata.

4. [OPTIONAL] Roll back the new module metadata.

   In case of failure, the setup can be reverted to the old approach.

   To do so, a PR can be opened to `/kyma/kyma-modules` reverting the submission from 2. ArgoCD then undeploys the new module metadata and KLM falls back to the old module metadata.

   > [!Note]
   > After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

5. Submit a version upgrade using the old format.

   To avoid version downgrades, prepare for failure recovery and submit a version update using the old metadata.

   - `/modules/telemetry/regular/module-config.yaml` pointing to `1.34.0` (before `1.32.0`)

   Since the new metadata exists, Lifecycle Manager continues to use it. The old metadata is ignored but remains available for rollback.

6. Submit the updated channel mapping with the NEW approach.

   After preparing the old metadata to rollback in case of failure, you can continue with the actual version update using the new metadata.

   Following the example, submit the `modules/telemetry/module-releases.yaml` file:

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

   Once you submit the mapping, the resources in `/kyma/kyma-modules` are the same as the resources from step 2, except the `regular` channel pointing to version `1.34.0` in all ModuleReleaseMeta CRs for different landscapes.

7. Verify if the module is updated in KCP.

   ArgoCD picks up this change and deploys the new ModuleReleaseMeta to the different landscapes. Lifecycle Manager is now picking up the version change and updating all modules using the `regular` channel to version `1.34.0`.

8. [OPTIONAL] Roll back the new metadata.

   In case of failure, you can revert the setup to the old approach.

   To do so, a PR can be opened to `/kyma/kyma-modules` reverting the submissions from steps 2 and 6. It is important to revert completely removing the entire new metadata from KCP so that KLM falls back to the old module metadata.

   > [!Note]
   > After rollback, you can still use the old submission pipeline to submit new versions of the module while you're working on a fix.

9. Delete all old metadata files related to the module.

10. Continue to use the NEW approach to provide new module versions and update the mapping of channels.
