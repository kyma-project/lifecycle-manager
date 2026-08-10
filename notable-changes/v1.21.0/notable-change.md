<!--
{
  "metadata": {
    "requirement": "MANDATORY",
    "type": "INTERNAL",
    "category": "MIGRATION"
  }
}
-->

# KLM: Remove the Purge Finalizer from All Kyma Custom Resources

> ### Caution:
> This migration is mandatory. Run the migration script immediately after updating Kyma Lifecycle Manager (KLM) to version 1.21.0 or greater. If you delay, any Kyma custom resource marked for deletion will remain stuck in the deletion process indefinitely. The purge controller that previously removed this finalizer no longer exists in KLM version 1.21.0.
 
## Prerequisites

Before you begin, ensure you have the following:

- `kubectl` configured to point to the KCP cluster.
- `yq` installed on the machine where you run the script.
- KLM updated to version 1.21.0 or greater in the `kcp-system` namespace.

## What's Changed

As of version 1.21.0, the purge controller has been removed from KLM. Previously, KLM added the finalizer `operator.kyma-project.io/purge-finalizer` to every Kyma custom resource. The finalizer coordinated forced cleanup of remote cluster resources during deprovisioning.

With the purge controller removed, nothing removes this finalizer anymore. Any Kyma custom resource that still carries it and enters deletion will block indefinitely. The resource waits for a finalizer owner that no longer exists.

## Procedure

1. Update KLM to version 1.21.0 or greater.

2. Immediately after the update, run the [migration script](./remove-purge-finalizer.sh).

   ```bash
   # Dry run first - shows affected Kyma custom resources without making any changes
   ./remove-purge-finalizer.sh

   # Apply the changes once you have verified the dry-run output
   ./remove-purge-finalizer.sh --execute
   ```

   The script reads the `app.kubernetes.io/version` label of the `klm-controller-manager` Deployment in the `kcp-system` namespace and warns if KLM has not yet been updated to at least 1.21.0. If the version check cannot be completed (for example, the label is missing or not in semver format), the script prompts you to confirm manually before proceeding.

3. You must answer with a capital `YES` for the script to proceed. Anything other than`YES` fails the script.

## Post-Update Steps

Verify that no Kyma custom resources in the cluster still carry the finalizer:

```bash
kubectl get kymas.operator.kyma-project.io --all-namespaces -o yaml \
  | yq '[.items[] | select(.metadata.finalizers[]? == "operator.kyma-project.io/purge-finalizer") | .metadata.name]'
```

The result must be an empty array (`[]`).
