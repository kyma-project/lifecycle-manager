# API Upgrade Guide for KCP-only Resources

## Phase 1: Collect breaking changes
Introduce new API labels on GH-issues for breaking API-changes.
- Collect issues by labels:
  - `component: api/kyma`, ` component: api/watcher`, etc. 
  - `version: v1beta3`
- Create epic for new version, planned in the appropriate sprint.

## Phase 2: Implement new version
Guide: https://book.kubebuilder.io/multiversion-tutorial/tutorial.html

- Create new version.
- Select the storage version to maximize data compatibility and consistency.
    - If removing fields, keep the old version as the storage version.
    - If adding fields, set the new version as the storage version.
    - If both removing and adding fields, keep the old version as the storage version, and add the new fields to both versions (fields in the old version must be optional).
- Create conversion webhook including conversion logic.
- Mark old version as deprecated but still served.
- Support CLI to generate new version.

## Phase 3: Deprecate old version
Guide: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#overview

- Set new version as storage version
- Deploy the change
- Make sure all existing resources are stored as new version
  - Run the [Storage Version migrator](https://github.com/kubernetes-sigs/kube-storage-version-migrator)
  - Remove the old version from the CustomResourceDefinition `status.storedVersions` field.
- Set old version served to `false`
- Deploy the change
- Ensure the [upgrade of existing objects to the new stored version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version) step has been completed:
  - Verify that the `storage` is set to `true` for the new version in the `spec.versions` list in the CustomResourceDefinition.
  - Verify that the old version is no longer listed in the CustomResourceDefinition `status.storedVersions`.
