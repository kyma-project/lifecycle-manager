# ADR 003 - Client Scope and Usage

## Status

Accepted

## Context

Our current application architecture includes direct references to the Kubernetes client interface across multiple layers, such as the Reconciler and the planned Service layers. This violates the separation of concerns and tightly couples orchestration and business logic to infrastructure-specific code.

To address this, we will adopt a 3-tier architecture (see [ADR 004](./004-layered-architecture.md)), where infrastructure dependencies like the Kubernetes client should only be referenced within the Repository layer, ensuring a clear boundary between data access and business logic.

We use dependency injection and interface-based programming to allow for mocking in tests. The Kubernetes client interface in use is provided by [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime/blob/6ad5c1dd4418489606d19dfb87bf38905b440561/pkg/client/interfaces.go#L164), specifically the `client.Client` interface. This interface is a composition of other interface definitions, `Reader`, `Writer`, `StatusClient`, and `SubResourceClientConstructor`as well as defining its own methods.

Given this, we have the following choices:
- Create our own simplified interfaces that align with our usage and ignore the embedded interfaces
- Use the `client.Client` interface directly in the Repository layer
- Create an internal interface that embeds a subset or all of the controller-runtime interfaces

This decision will influence how we encapsulate data access logic aka kube API access and how flexible our system will be for testing.

## Decision

We will use the `client.Client` interface provided by controller-runtime directly in the Repository layer and avoid creating custom interfaces or client implementations. This approach minimizes unnecessary abstraction and leverages the mature, well-tested interfaces provided by the Kubernetes ecosystem.

Whenever possible, we will reference the most specific sub-interface from the `client.Client` composition (e.g., `Reader`, `Writer`) rather than the full `Client` interface. This promotes better adherence to the Interface Segregation Principle and allows for more precise dependency injection, leading to simpler and more focused unit tests. If methods from multiple sub-interfaces are needed, the `client.Client` is used.

The client will **only** be referenced in the Repository layer. All other layers, such as Service and Reconciler, will remain decoupled from infrastructure concerns and interact only through higher-level abstractions defined in the application domain.

### Do's

These configuration options are compliant with the decision:

A Service defines its dependency as `[Prefix]Repository`. The Repository implementation then uses controller-runtime's Client interfaces directly.
```go
// package internal/service/foo
type Service struct {
    barRepository BarRepository
}

type BarRepository interface {
    Get(name, namespace string) (*v1beta2.Manifest, error)
}
```

```go
// package internal/repository/bar
type Repository struct {
    readClient client.Reader
}
```

Another example, for more than only read:
```go
// package internal/service/foo
type BarRepository interface {
    Get(name, namespace string) (*v1beta2.Manifest, error)
    Create(name, namespace string) error
    Update(*v1beta2.Manifest) error
}
```

```go
// package internal/repository/bar
type Repository struct {
    kcpClient client.Client
}
```

### Don'ts

These configuration options are non-compliant with the decision:

1. The Service defines a dependency as `[Prefix]Client` and uses it:
    ```go 
    type Service struct {
        barClient BarClient
    }

    type ManifestClient interface {
        Get(name, namespace string) (*v1beta2.Manifest, error)
    }
    ```
    Mitigation: The dependency must be called `*Repository`.

2. The Service consumes the defined ManifestRespository interface as an embedded field:
    ```go
    type Service struct {
        BarRepository
    }

    type BarRepository interface {
        Get(name, namespace string) (*v1beta2.Manifest, error)
    }
    ```
    Mitigation: The `BarRepository` interface must be referenced as a named, private field so it can not be accessed across the package.

3. The Repository implementation defines its own intermediate composition interface from controller-runtime interfaces:
    ```go
    type Repository struct {
        barClient barClient
    }

    type barClient interface {
        client.Writer
        client.Reader
    }
    ```
    Mitigation: The Repository implementation must use the `client.Client` directly if more than one sub-interface is needed.


## Consequences

- The Repository layer becomes the single point of interaction with the Kubernetes API, improving separation of concerns and making the system easier to reason about.
- By using specific sub-interfaces (e.g., Reader, Writer) when possible, dependencies are minimized, and unit testing becomes simpler and more focused.
- Higher layers (Service, Reconciler) remain infrastructure-agnostic, promoting separation of concerns and better testability.
- We rely directly on the controller-runtime client interfaces, which reduces boilerplate but ties our implementation closely to that library’s abstractions.
