# ADR 002 - Constructor Injection and Composition Functions

## Status

Accepted

## Context

It is agreed that [Dependency Inversion](https://medium.com/@inzuael/solid-dependency-inversion-principle-part-5-f5bec43ab22e) is a key principle for solid architecture.
Once a consumer abstracted their dependencies properly, the key question is how to fill the abstraction with a concrete implementation.

It has to be decided how and where to resolve dependencies.

## Decision

It is decided that a consumer must have its dependencies injected via a constructor. In addition:
* Constructors should follow the `New<struct name>` naming pattern .
* Stable dependencies of a consumer are resolved in a *composition function*.
* Composition functions live in a separate package at `cmd/composition`.
* Composition functions should follow the `Compose<struct name>` naming pattern.
* Composition functions should build stable dependencies of the unit to compose themselves and have other dependencies injected from the outside.
* Composition functions do NOT return errors, but instead, log them and exit the startup process.
* Composition functions are called from the `main` function, and the dependency tree is resolved.

## Consequence

* Consumers do not build or know the concrete implementations of their abstracted dependencies.
* The `main` function is kept free from resolving the detailed dependency tree.
* The detailed dependency tree is resolved via the composition functions under `cmd/composition`.

As the above decisions are applied, they may be continuously refined and extended.

### Example

```go
// /cmd/composition/service/webhook/skr_webhook_manager.go
package webhook

import (
  "github.com/kyma-project/lifecycle-manager/internal/watcher"
  "github.com/kyma-project/lifecycle-manager/internal/watcher/certmanager"
)

func ComposeSkrWebhookManager(logger logr.Logger, flagVar *flags.FlagVar) *watcher.SkrWebhookManager {
  skrWebhookManager, err := watcher.NewSKRWebhookManager(
    certmanager.NewCertificateManager(
      certmanager.CertificateManagerConfig{
        CertificateNamespace: flagVar.SelfSignedCertificateNamespace
      }
    ),
    watcher.SkrWebhookManagerConfig{
      SkrWatcherPath:   flagVar.WatcherResourcesPath,
      SkrWatcherImage:  flagVar.GetWatcherImage(),
    },
  )

  if err != nil {
    logger.Error(err, "failed to compose SKRWebhookManager")
    os.Exit(bootstrapFailedExitCode)
  }

  return skrWebhookManager
}
```

```go
// /cmd/main.go
package main

import (
  "github.com/kyma-project/lifecycle-manager/cmd/composition/reconciler"
)

func main() {
  // ...

  kymaReconciler := kyma.NewReconciler(
    webhook.ComposeSkrWebhookManager(logger, flagVar)
  )

  kymaReconciler.SetupWithManager(
    mgr,
    opts,
    settings,
  )
}
```
