# Documentation

## Overview

The `docs` folder contains documentation on the Lifecycle Manager project.

## Table of contents

The table of contents lists all the documents in repository with their short description.

- [Developer tutorials](./developer-tutorials/README.md) - a directory containing infrastructure-related guides for developers
  - [Provide credential for private OCI registry authentication](./developer-tutorials/config-private-registry.md)
  - [Local test setup in the control-plane mode using k3d](./developer-tutorials/local-test-setup.md)
  - [Create a test environment on Google Container Registry (GCR)](./developer-tutorials/prepare-gcr-registry.md)
  - [Provision cluster and OCI registry](./developer-tutorials/provision-cluster-and-registry.md)
  - [Enable Webhooks in Lifecycle Manager](./developer-tutorials/starting-operator-with-webhooks.md)
- Technical reference - a directory with techncial details on Lifecycle Manager, such as architecture, APIs, or running modes
  - [API](./technical-reference/api/README.md) - a directory with the description of Lifecycle Manager's custom resources (CRs)
    - [Kyma CR](./technical-reference/api/kyma-cr.md)
    - [Manifest CR](./technical-reference/api/manifest-cr.md)
    - [ModuleTemplate CR](./technical-reference/api/moduleTemplate-cr.md)
  - [Architecure](./technical-reference/architecture.md) - describes Lifecycle Manager's architecture
  - [Controllers](./technical-reference/controllers.md) - describes Kyma, Manifest and Watcher controllers
  - [Running Modes](./technical-reference/running-modes.md) - describes Lifecycle Manager's running modes
  - [Declarative Reconciliation Library Reference Documentation](../internal/declarative/README.md)
  - [Internal Manifest Reconciliation Library Extensions](../internal/manifest/README.md)
  - [Smoke tests](../tests/smoke_test/README.md)
- User tutorials
  - [Managing module enablement with the CustomResourcePolicy](./user-tutorials/02-10-manage-module-with-custom-resource-policy.md)
  - [Quick Start](./user-tutorials/01-10-control-plane-quick-start.md)
- [Modularization](modularization.md) - describes the modularization concept and its building blocks in the context of Lifecycle Manager
