# Upgrading Kyma Control Plane Resources API

## Phase 1: Collecting Breaking Changes
1. Introduce new API labels for API breaking changes in GitHub issues.
2. Collect issues by labels:
  - `component: api/kyma`, ` component: api/watcher`, etc. 
  - `version: v1beta3`
3. Create an epic for the new API version planned for the appropriate sprint.

## Phase 2: Implement the New API Version
Guide: https://book.kubebuilder.io/multiversion-tutorial/tutorial.html

1. Create the new API version.
2. Select the storage version to maximize data compatibility and consistency.
    - If removing fields, keep the old version as the storage version.
    - If adding fields, set the new version as the storage version.
    - If both removing and adding fields, keep the old version as the storage version, and add the new fields to both versions (fields in the old version must be optional).
3. Create a conversion webhook including conversion logic.
4. Mark the old version as deprecated but still served.
5. Support CLI to generate the new version.

## Phase 3: Deprecate the Old Version
Guide: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#overview

1. Set the new API version as the storage version.
   a. Deploy the change.
   b. Make sure all existing resources are stored as the new version.
   c. Run the [Storage Version migrator](https://github.com/kubernetes-sigs/kube-storage-version-migrator)
   d. Remove the old version from the CustomResourceDefinition's `status.storedVersions` field.
2. Set the old version served to `false`.
3. Deploy the change.
4. Ensure the [upgrade of existing objects to the new stored version](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version) step has been completed:
   a. Verify that the `storage` is set to `true` for the new version in the `spec.versions` list in the CustomResourceDefinition.
   b. Verify that the old version is no longer listed in the CustomResourceDefinition's `status.storedVersions`.
