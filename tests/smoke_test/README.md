# Smoke Tests

This Subdirectory contains smoke tests used for E2E Verification.

It has 3 main goals:

- Be fast enough to execute in PRs
- Be small enough to quickly iterate
- Be comprehensive enough to serve as a middle-ground between integration tests and e2e tests

## Contents

This Repo contains a `Makefile` which will execute `go test` against a smoke-test running
with [The official Kubernetes E2E Testing Framework](https://github.com/kubernetes-sigs/e2e-framework).

It will also use a downloaded version of kustomize and the Kyma CLI to properly test its workflows.

## Prerequisites for running the Smoke Tests

1. A unix HOST OS
2. Support for the Docker Socket for the Provisioning Parts

## Run the Tests

Simply run `make` and let the magic happen!