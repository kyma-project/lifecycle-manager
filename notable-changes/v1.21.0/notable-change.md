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
> This migration is mandatory. Run the migration script immediately after updating KLM to v1.21.0 or greater. If you delay running the script, any Kyma custom resource that is marked for deletion will be stuck in the deletion process indefinitely, because the purge controller that previously removed this finalizer no longer exists in KLM v1.21.0.

## Prerequisites

- `kubectl` configured to point to the KCP cluster
- `yq` installed on the machine where you run the script
- KLM updated to v1.21.0 or greater in the `kcp-system` namespace

## What's Changed

As of v1.21.0, the purge controller has been removed from KLM. Previously, KLM added the finalizer `operator.kyma-project.io/purge-finalizer` to every Kyma custom resource to coordinate forced cleanup of remote cluster resources during deprovisioning.

With the purge controller removed, nothing removes this finalizer anymore. Any Kyma custom resource that still carries it and enters deletion will block indefinitely, waiting for a finalizer owner that no longer exists.

## Procedure

1. Update KLM to v1.21.0 or greater.

2. Immediately after the update, run the migration script from the KLM repository root:

   ```bash
   # Dry run first - shows affected Kyma custom resources without making any changes
   ./scripts/remove-purge-finalizer.sh --min-version v1.21.0

   # Apply the changes once you have verified the dry-run output
   ./scripts/remove-purge-finalizer.sh --min-version v1.21.0 --execute
   ```

   The script performs a sanity check at startup: it reads the `app.kubernetes.io/version` label of the `controller-manager` Deployment in the `kcp-system` namespace and aborts if KLM has not yet been updated to at least v1.21.0. This prevents accidentally running the script against an old KLM version that would re-add the finalizer.

   If the deployed version label is not in semver format, the script exits with a warning. In that case, after manually confirming that the deployed version is v1.21.0 or greater, run the script with `--skip-version-check`:

   ```bash
   ./scripts/remove-purge-finalizer.sh --skip-version-check --execute
   ```

## Post-Update Steps

Verify that no Kyma custom resources in the cluster still carry the finalizer:

```bash
kubectl get kymas.operator.kyma-project.io --all-namespaces -o yaml \
  | yq '[.items[] | select(.metadata.finalizers[]? == "operator.kyma-project.io/purge-finalizer") | .metadata.name]'
```

The result must be an empty array (`[]`).
