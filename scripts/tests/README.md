# Overview of All Scripts

## Version Checker: `version.sh`
The script checks if the Command Line Tools (CLI) for `kubectl`, `docker`, `GoLang`, `k3d`, and `istioctl` have the correct versions.
It ensures that the versions are in a valid format using [Semantic Versioning](https://semver.org/).
If the check detects outdated versions, it gives a warning and exits with success. For more information, see all the possible exit statuses:  

* `0` - Success - the version is up-to-date or outdated and it uses the correct Semantic Versioning
* `1` - At least one of the CLI tools is not installed
* `2` - Invalid version found, for example, incorrect Semantic Versioning

### Current Versions

The current tooling versions are defined in [`versions.yaml`](../../versions.yaml).

## Create Test Clusters: `create_test_cluster.sh`
The script creates two test clusters using `k3d`:
- `kcp` for the control plane 
- `skr` for the Kyma runtime

If the clusters already exist, the script exits with success.
If you get a notification, while running the script, saying that the Watcher CR is missing, re-run the script.
The script is accompanied by the `Ensure Test Clusters` task in the VSCode tasks, and by the corresponding run configuration for GoLand.

It requires the following parameters:
- `--k8s-version`: The version of K8s to use. Must be a semantic version.
- `--cert-manager-version`: The version of cert-manager to use. Must be a semantic version.

This script internally depends on `version.sh` to check the versions of the required tools. If you want to skip the version check, use the `--skip-version-check` flag.

## Cleaning the Clusters: `clusters_cleanup.sh`
The script deletes the `kcp` and `skr` test clusters using `k3d`.

## Deploying KLM from Sources: `deploy_klm_from_sources.sh`
The script deploys Lifecycle Manager using the current state of the locally cloned and developed repository.
It doesn't require any additional flags or parameters.

## Deploying KLM from the Registry: `deploy_klm_from_registry.sh`
The script deploys Lifecycle Manager from the given image registry and the given image tag.
It requires the following parameters:
- `--image-registry`: The accepted values are `prod` and `dev`.
- `--image-tag`: The tag of the image to be used. For example, `latest`.

## Deploy Kyma: `deploy_kyma.sh`
Use the script to deploy Kyma using one of the **required** parameters:
- `localhost`: To run Lifecycle Manager locally on your machine.
- `host.k3d.internal`: To deploy Lifecycle Manager to a cluster.

## Undeploy Kyma: `undeploy_kyma.sh`
The script undeploys Kyma from the cluster by deleting the Kyma and the corresponding Secret from the `kcp-system` namespace.

## End-To-End Tests: `e2e.sh`
The script runs end-to-end tests taking the test target as input.
The test targets are defined in the `tests` directory of the project root.
The script runs the test target and outputs the results to the console.
The errors that occurred during the test are handled directly by `make`.

## Installing CRDs: `install_crds.sh`
The script installs Custom Resource Definitions (CRDs) required for Lifecycle Manager. The CRDs' set is the same as in the `make install` of the Makefile in the project root directory.
