# Notable Changes

Notable changes are Kyma Lifecycle Manager (KLM) updates that require operator action. They are documented and delivered to operators as part of a SAP BTP, Kyma runtime release.

## Classification

- **Requirement**:
  - **Mandatory** — Operator action is required for proper functionality.
  - **Recommended** — Operator action is recommended but not strictly required.

- **Type**:
  - **External** — Customer-facing change that affects user experience.
  - **Internal** — Operator-facing change that impacts internal processes.

- **Category**:
  - **Configuration** — Updates that require configuration adjustments.
  - **Feature** — A new feature that requires operator action before or during deployment.
  - **Migration** — Changes that involve data, infrastructure, or version migrations.

## Creating a Notable Change

When introducing a KLM change that requires operator action:

1. In the notable-changes directory, create a folder named with the next KLM version number (for example, `1.17.0`).

2. In that folder, create a `notable-change.md` file using the [Notable Change Template](./assets/notable-change-template.md).

3. Fill in the JSON metadata block at the top of the file:

   ```json
   {
     "metadata": {
       "requirement": "RECOMMENDED",
       "type": "INTERNAL",
       "category": "CONFIGURATION"
     }
   }
   ```

   Valid values:
   - **requirement**: `MANDATORY` or `RECOMMENDED`
   - **type**: `EXTERNAL` or `INTERNAL`
   - **category**: `CONFIGURATION`, `FEATURE`, or `MIGRATION`

4. Set the document title using the format: **"KLM: `<Name of the update>`"**.

5. Clearly describe the impact, required actions, and any other relevant details.

6. Include any supporting files (migration scripts, configuration examples) in the same folder.

## Publication

KCP components listed in `product-kyma-runtime/kcp/component-version.yaml` are automatically scanned for new notable changes. Files from the `notable-changes` directory are pulled into the `docs/02-10-update` directory of the `product-kyma-runtime` repository and aggregated by release version.

The packages are automatically uploaded to the Help Portal as part of the **SAP BTP, Kyma Runtime Operator's Guide** under the **Update** section.
