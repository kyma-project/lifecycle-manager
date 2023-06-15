# E2e Tests

This Subdirectory contains e2e tests used for E2E Verification.
Those tests are written to run inside a pipeline. But they can also be run on a local machine, as described in the next section.

## Content

- [Test  Suite](./e2e_suite_test.go): Sets up needed clients and configurations.
- [Watcher end-to-end tests](./e2e_watcher_test.go): Tests the end-to-end workflow of the watcher.

## Run the Tests locally

1. Setup local environment using [this guide](../../docs/developer/local-test-setup.md). Only cluster creation and pre-requisites steps are needed, rest can be skipped. 
2. Export K8s Configurations into environment variables
    ```shell
    export KCP_KUBECONFIG=$(k3d kubeconfig write kcp-local)
    export SKR_KUBECONFIG=$(k3d kubeconfig write skr-local)
    ```
3. Run `make`
