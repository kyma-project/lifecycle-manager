# ADR 002 - Constructor Injection and Composition Functions

## Status

Accepted

## Context

It is agreed that [Depenency Inversion](https://medium.com/@inzuael/solid-dependency-inversion-principle-part-5-f5bec43ab22e) is a key principle for solid architecture.
Once a consumer abstracted their dependencies properly, the key question is how to fill the abstraction with a concrete implementation.

It has to be decided how and where to resolve depdenencies.

## Decision

It is decided that a consumer must have its depenencies injected via constructor.
Constructors should follow the naming pattern `New<struct name>`.
Dependencies of a consumer are resolved in a *composition function*.
Composition functions live in a separate pacakge at `cmd/composition`.
Composition functions should follow the naming pattern `Compose<struct name>`.
Composition functions may build depenencies of the unit to compose themselves or have them injected from outside (depends on the kind of dependency).
Composition functions do NOT return errors but instead log them and exit the startup process.
Composition functions are called from `main` and import the depenencies at the highest level needed.

## Consequence

Consumers do not build or know the concrete implementations of their abstracted dependencies.
The `main` function is kept free from resolving the detailed dependency tree.
The detailed dependency tree is resolved via the compositions functions under `cmd/composition`.

As the above decisions are applied, they may be continuously refined and extended.

### Example

```go
// /cmd/composition/service/webhook/skr_webhook_manager.go
package webhook

import (
  "github.com/kyma-project/lifecycle-manager/internal/watcher"
)

func ComposeSkrWebhookManager(logger logr.Logger, flagVar *flags.FlagVar) *watcher.SkrWebhookManager {
  certManager := composeCertManager(logger, flagVar)

  config := watcher.SkrWebhookManagerConfig{
    SkrWatcherPath:   flagVar.WatcherResourcesPath,
    SkrWatcherImage:  flagVar.GetWatcherImage(),
  }

  skrWebhookManager, err := watcher.NewSKRWebhookManager(certManager, config)
  if err != nil {
    logger.Error(err, "failed to compose SKRWebhookManager")
    os.Exit(bootstrapFailedExitCode)
  }

  return skrWebhookManager
}
```

```go
// /cmd/composition/reconciler/kyma.go
package reconciler

import (
  "github.com/kyma-project/lifecycle-manager/cmd/composition/service/webhook"
  "github.com/kyma-project/lifecycle-manager/internal/controller/kyma"
)

func ComposeKymaReconciler(logger logr.Logger, flagVar *flags.FlagVar) *kyma.Reconciler {
  return kyma.NewReconciler(
    webhook.ComposeSkrWebhookManager(logger, flagVar)
  )
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

  kymaReconciler := reconciler.ComposeKymaReconciler(logger, flagVar)
  kymaReconciler.SetupWithManager(
    mgr,
    opts,
    settings,
  )
}
```
