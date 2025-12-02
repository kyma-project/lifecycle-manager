# Creating ModuleTemplate with modulectl and OCM CLI

This guide describes how to create ModuleTemplate custom resources using [modulectl](https://github.com/kyma-project/modulectl) in combination with [OCM CLI](https://github.com/open-component-model/ocm).

## Overview

ModuleTemplate creation involves two main steps:
1. **Generate component descriptor and ModuleTemplate manifest** using modulectl
2. **Push component descriptor to OCI registry** using OCM CLI

At runtime, Kyma Lifecycle Manager fetches component descriptors dynamically from the OCI registry based on the `ModuleReleaseMeta.Spec.OcmComponentName` and `ModuleTemplate.Spec.Version`.

## Prerequisites

- [modulectl](https://github.com/kyma-project/modulectl) installed
- [OCM CLI](https://github.com/open-component-model/ocm) installed
- Access to an OCI registry (local or remote)

## 1. Configure OCM Registry Access

Create an OCM configuration file to define registry aliases and enable communication with your target registry.

### For Local Registries

If using a local registry (e.g., `localhost:5111`):

```yaml
# ocm-config-local.yaml
type: generic.config.ocm.software/v1
configurations:
  - type: ocm.config.ocm.software
    aliases:
      localhost:5111:
        type: OCIRegistry
        baseUrl: http://localhost:5111
  - type: oci.config.ocm.software
    aliases:
      localhost:5111:
        type: OCIRegistry
        baseUrl: http://localhost:5111
```

### For Private Registries with Authentication

```yaml
# ocm-config-private.yaml
type: generic.config.ocm.software/v1
configurations:
  - type: ocm.config.ocm.software
    aliases:
      europe-docker.pkg.dev:
        type: OCIRegistry
        baseUrl: https://europe-docker.pkg.dev
  - type: credentials.config.ocm.software
    consumers:
      - identity:
          type: OCIRegistry
          hostname: europe-docker.pkg.dev
        credentials:
          - type: Credentials
            properties:
              username: _json_key
              password: |
                {
                  "type": "service_account",
                  ...
                }
```

## 2. Generate Component Constructor

Use `modulectl create` to generate the component constructor file without pushing to registry:

```bash
modulectl create \
  --config-file module-config.yaml \
  --disable-ocm-registry-push \
  --output-constructor-file component-constructor.yaml
```

**Key parameters:**
- `--config-file`: Path to your module configuration (defines resources, images, etc.)
- `--disable-ocm-registry-push`: Generate component constructor without pushing to registry
- `--output-constructor-file`: Output file for the component constructor

This generates:
- `component-constructor.yaml` - OCM component constructor file
- `template.yaml` - The ModuleTemplate CR to apply to the cluster

## 3. Add Component Version to CTF Archive

Add the component version from the constructor file to a Common Transport Format (CTF) archive:

```bash
ocm --config ocm-config-local.yaml add componentversions \
  --create \
  --file ./component-ctf \
  --skip-digest-generation \
  component-constructor.yaml
```

**Parameters:**
- `--config`: Path to OCM configuration file (for registry access)
- `add componentversions`: Add component versions to the CTF archive
- `--create`: Create the CTF archive directory if it doesn't exist
- `--file`: Path where CTF archive will be created (directory)
- `--skip-digest-generation`: Skip automatic digest generation (for faster processing)
- Final argument: Path to the component constructor file

This creates a CTF archive in the `./component-ctf` directory containing the component descriptor and all referenced resources.

## 4. Transfer CTF to OCI Registry

Transfer the complete CTF archive to the OCI registry:

```bash
ocm --config ocm-config-local.yaml transfer ctf \
  --overwrite \
  --no-update \
  ./component-ctf \
  http://localhost:5111
```

**Parameters:**
- `--config`: Path to OCM configuration file
- `transfer ctf`: Transfer command for CTF archives
- `--overwrite`: Overwrite existing component versions in the registry
- `--no-update`: Don't update component references during transfer
- `./component-ctf`: Path to the source CTF archive directory
- `http://localhost:5111`: Target OCI registry URL

This pushes the component descriptor and all resources from the CTF to the OCI registry.

## 5. Apply ModuleTemplate to Cluster

Apply the generated ModuleTemplate manifest:

```bash
yq -i '.metadata.namespace="kcp-system"' template.yaml
kubectl apply -f template.yaml
```

## 6. Create ModuleReleaseMeta

Create a ModuleReleaseMeta CR to make the module available:

```yaml
apiVersion: operator.kyma-project.io/v1beta2
kind: ModuleReleaseMeta
metadata:
  name: template-operator
  namespace: kcp-system
spec:
  moduleName: template-operator
  ocmComponentName: kyma-project.io/module/template-operator
  channels:
    - channel: regular
      version: 1.0.4
```

Apply it:

```bash
kubectl apply -f module-release-meta.yaml
```

## Runtime Behavior

When a Kyma CR requests a module:
1. Lifecycle Manager reads `ModuleReleaseMeta` to get the OCM component name and version
2. Fetches the component descriptor from the OCI registry using these identifiers
3. Parses the descriptor to extract image references and resources
4. Deploys the module based on the descriptor content

> **Note:** The `spec.descriptor` field in ModuleTemplate is deprecated and not used at runtime. Component descriptors are always fetched from the OCI registry.

## See Also

- [ModuleTemplate CR Reference](resources/03-moduletemplate.md)
- [modulectl Documentation](https://github.com/kyma-project/modulectl)
- [OCM CLI Documentation](https://github.com/open-component-model/ocm)
