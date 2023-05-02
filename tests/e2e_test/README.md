# E2e Tests

This Subdirectory contains e2e tests used for E2E Verification.


## Contents

This Repo contains a `Makefile` which will execute `go test` against a e2e running
with [The official Kubernetes E2E Testing Framework](https://github.com/kubernetes-sigs/e2e-framework).

It will also use a downloaded version of kustomize and the Kyma CLI to properly test its workflows.

## Run the Tests

1. Setup local environment using [this guide](../../docs/developer/local-test-setup.md)
2. Export K8s Configurations into environment variables
    ```shell
    export KCP_KUBECONFIG=$(k3d kubeconfig write kcp-local)
    export SKR_KUBECONFIG=$(k3d kubeconfig write skr-local)
    ```
3. Run `make`
