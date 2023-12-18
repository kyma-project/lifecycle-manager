# E2e Tests

This subdirectory contains e2e tests used for the e2e verification of the watcher component.
Those tests are written to run inside a pipeline. But they can also be run on a local machine.
See the [Run the e2e tests locally](#run-the-e2e-tests-locally) section for details.

## Contents

- [Test suite](./suite_test.go) that sets up required clients and configurations.
- [Watcher end-to-end tests](./watcher_test.go) that includes tests and the end-to-end workflow of the watcher.
- [Makefile](./Makefile) that includes targets to execute the watcher e2e test suite.

## Run the e2e tests locally

1. Set up your local environment using steps 1 to 3 from [this guide](/docs/developer-tutorials/local-test-setup.md). Create a cluster, and install Istio CRDs and `cert-manager`. Skip all the remaining steps from the guide. 
2. Export the following  K8s configurations as environment variables:
    ```shell
    export KCP_KUBECONFIG=$(k3d kubeconfig write kcp-local)
    export SKR_KUBECONFIG=$(k3d kubeconfig write skr-local)
    ```
3. Run `make watcher-enqueue`.
