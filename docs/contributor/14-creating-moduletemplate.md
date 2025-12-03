# Creating ModuleTemplate with modulectl and OCM CLI

This guide describes how to create ModuleTemplate custom resources using [modulectl](https://github.com/kyma-project/modulectl) in combination with [OCM CLI](https://github.com/open-component-model/ocm).

## Overview

ModuleTemplate creation involves two main steps:

1. Generate a component descriptor and ModuleTemplate manifest using modulectl.
2. Push the component descriptor to the OCI registry** using OCM CLI.

At runtime, Kyma Lifecycle Manager fetches component descriptors dynamically from the OCI registry based on the `ModuleReleaseMeta.Spec.OcmComponentName` and `ModuleTemplate.Spec.Version`.

## Prerequisites

- [modulectl](https://github.com/kyma-project/modulectl) installed
- [OCM CLI](https://github.com/open-component-model/ocm) installed
- Access to an OCI registry (local or remote)

## Procedure

1. Create an OCM configuration file to define registry aliases and enable communication with your target registry.

   a. For local registries, such as `localhost:5111`, use:

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

   b. For private registries with authentication, use:

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

2. Use `modulectl create` to generate the component constructor file without pushing to the registry.

   ```bash
   modulectl create \
     --config-file module-config.yaml \
     --disable-ocm-registry-push \
     --output-constructor-file component-constructor.yaml
   ```

   **Key parameters:**
    - `--config-file`: The path to your module configuration (defines resources, images, etc.)
    - `--disable-ocm-registry-push`: Generates component constructor without pushing to registry
    - `--output-constructor-file`: The output file for the component constructor

   The command generates:
    - `component-constructor.yaml`: The OCM component constructor file
    - `template.yaml`: The ModuleTemplate CR to apply to the cluster

3. Add the component version from the constructor file to a Common Transport Format (CTF) archive.

   ```bash
   ocm --config ocm-config-local.yaml add componentversions \
     --create \
     --file ./component-ctf \
     --skip-digest-generation \
     component-constructor.yaml
   ```

   **Key parameters:**
    - `--config`: The path to the OCM configuration file (for registry access)
    - `add componentversions`: Adds component versions to the CTF archive
    - `--create`: Creates the CTF archive directory if it doesn't exist
    - `--file`: The path where the CTF archive is created (directory)
    - `--skip-digest-generation`: Skips automatic digest generation (for faster processing)
    - `component-constructor.yaml`: The path to the component constructor file

   The command creates a CTF archive in the `./component-ctf` directory containing the component descriptor and all referenced resources.

4. Transfer the complete CTF archive to the OCI registry.

   ```bash
   ocm --config ocm-config-local.yaml transfer ctf \
     --overwrite \
     --no-update \
     ./component-ctf \
     http://localhost:5111
   ```

   **Key parameters:**
    - `--config`: The path to the OCM configuration file
    - `transfer ctf`: Transfers command for CTF archives
    - `--overwrite`: Overwrites the existing component versions in the registry
    - `--no-update`: Prevents updating component references during transfer
    - `./component-ctf`: The path to the source CTF archive directory
    - `http://localhost:5111`: The target OCI registry URL

   The command pushes the component descriptor and all resources from the CTF to the OCI registry.

5. Apply the generated ModuleTemplate manifest in the cluster.

   ```bash
   yq -i '.metadata.namespace="kcp-system"' template.yaml
   kubectl apply -f template.yaml
   ```

6. Create a ModuleReleaseMeta CR to make the module available.

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

   To apply the CR, run:

   ```bash
   kubectl apply -f module-release-meta.yaml
   ```

## Runtime Behavior

When a Kyma CR requests a module, Lifecycle Manager:
1. Reads `ModuleReleaseMeta` to get the OCM component name and version.
2. Fetches the component descriptor from the OCI registry using these identifiers.
3. Parses the descriptor to extract image references and resources.
4. Deploys the module based on the descriptor content.

> ### Note
> The `spec.descriptor` field in ModuleTemplate is deprecated and not used at runtime. Component descriptors are always fetched from the OCI registry.

## Related Information

- [ModuleTemplate CR Reference](./resources/03-moduletemplate.md)
- [modulectl Documentation](https://github.com/kyma-project/modulectl)
- [OCM CLI Documentation](https://github.com/open-component-model/ocm)
