# Configure a Local Test Setup (VS Code & GoLand)

## Overview

Learn how to quickly provision and manage a local end-to-end test environment using Visual Studio Code or GoLand. You get ready-to-use scripts and IDE launch configurations for a seamless developer experience.

## Prerequisites

You have installed the following tools (for the required versions, see [`versions.yaml`](../../versions.yaml)):
- [Docker](https://www.docker.com/)
- [Go](https://go.dev/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [k3d](https://k3d.io/stable/)
- [yq](https://github.com/mikefarah/yq)
- [VS Code](https://code.visualstudio.com/) or [GoLand](https://www.jetbrains.com/go/)


## Using Visual Studio Code

1. **Open the Project in VS Code**

   Open the project root folder in VS Code.

2. **Use Predefined Tasks**

   - To access the predefined tasks as defined in `.vscode/tasks.json`, press `Cmd+Shift+P` and type `Tasks: Run Task`. 
   - You see tasks like `Create New Test Clusters`, `Install CRDs`, and `Deploy Lifecycle Manager`.

3. **Provision a Dual Cluster Test Infrastructure**

   To set up a dual cluster environment (KCP and SKR) with KLM and the template-operator module, run the following tasks in order:

    1. **Create New Test Clusters**  
      Provisions fresh KCP and SKR clusters with the specified Kubernetes and cert-manager versions.

    1. **Deploy KLM from sources**  
      Installs the Lifecycle Manager (KLM) and its dependencies into the KCP cluster from local sources.

    1. **Deploy template-operator**  
      Deploys the selected `ModuleTemplate` manifest, which includes the template-operator module, into the KCP cluster.

    1. **Deploy kyma**  
      Installs the Kyma custom resource into the SKR cluster, connecting it to the KCP cluster.

   After running these predefined tasks, your environment has KLM and all required components in KCP (with context `k3d-kcp`), and a ModuleTemplate with the template-operator ready for local testing, as well as the SKR with context `k3d-skr`.


## Using GoLand

1. **Open the Project in GoLand**

   Open the project root folder in GoLand.

2. **Use Run/Debug Configurations**

   - GoLand automatically detects configurations from the `.run` directory.
   - Open the Run/Debug Configurations dialog and select a configuration (such as `Create Test Clusters` or `Install CRDs`).

3. **Run or Debug**

   - To execute the selected configuration, choose **Run or Debug** .
   - The configuration runs the associated script or Go test.

## Common Tasks

| Task                        | VS Code Task Name            | GoLand Run Config Name      | Script Executed                                 |
|-----------------------------|-----------------------------|----------------------------|-------------------------------------------------|
| Create New Test Clusters    | Create New Test Clusters    | Create Test Clusters        | `./scripts/tests/create_test_clusters.sh`        |
| Ensure Test Clusters        | Ensure Test Clusters        | Ensure Test Clusters        | `./scripts/tests/create_test_clusters.sh`        |
| Delete Test Clusters        | Delete Test Clusters        | Delete Test Clusters        | `./scripts/tests/clusters_cleanup.sh`            |
| Deploy KLM from sources     | Deploy KLM from sources     | Deploy KLM from sources     | `./scripts/tests/deploy_klm_from_sources.sh`     |
| Deploy KLM from registry    | Deploy KLM from registry    | Deploy KLM from registry    | `./scripts/tests/deploy_klm_from_registry.sh`    |
| Deploy template-operator    | Deploy template-operator    | Deploy template-operator    | `kubectl apply -f ./tests/e2e/moduletemplate/...`|
| Deploy kyma                 | Deploy kyma                 | Deploy kyma                 | `./scripts/tests/deploy_kyma.sh`                 |
| Un-Deploy kyma              | Un-Deploy kyma              | Un-Deploy kyma              | `./scripts/tests/undeploy_kyma.sh`               |
| E2E Tests                   | E2E Tests                   | E2E Tests                   | `./scripts/tests/e2e.sh`                         |
| Install CRDs                | Install CRDs                | Install CRDs                | `./scripts/tests/install_crds.sh`                |

## Task Inputs

Some tasks prompt for input, such as:
- Kubernetes version (`k8sVersion`)
- cert-manager version (`certManagerVersion`)
- E2E test target (`e2eTestTarget`)
- Template-operator manifest (`templateOperatorVersion`)
- KLM image registry/tag (`klmImageRegistry`, `klmImageTag`)
- SKR host (`skrHost`)

These are automatically handled by VS Code when running the tasks.

## Tips

- All scripts are located in the `scripts/tests` directory.
- For advanced debugging, use breakpoints and the integrated terminal in your IDE.
- For troubleshooting, check the output panel or terminal for script logs.

With this setup, you can provision, test, and clean up your local environment with a single click or command, using your preferred IDE.