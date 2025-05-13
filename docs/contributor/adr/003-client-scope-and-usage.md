# ADR 003 - Client scope and usage

## Status

Accepted

## Context

Our current application architecture includes direct references to the Kubernetes client interface across multiple layers, such as the Reconciler and to be Service layers. This leads to a violation of the separation of concerns and tightly couples orchestration and business logic to infrastructure-specific code.

To address this, we will adopt a 3-tier architecture (see ADR 004), where infrastructure dependencies like the Kubernetes client should only be referenced within the Repository layer, ensuring a clear boundary between data access and business logic.

We use dependency injection and interface-based programming to allow for mocking in tests. The Kubernetes client interface in use is provided by [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime/blob/6ad5c1dd4418489606d19dfb87bf38905b440561/pkg/client/interfaces.go#L164), specifically the `client.Client` interface. This interface is a composition of other interface definitions, `Reader`, `Writer`, `StatusClient`, and `SubResourceClientConstructor`as well as defining own methods.

Given this, we have choices:
- Create our own simplified interfaces that align with our usage and ignore the embedded interfaces
- Use the client.Client interface directly in the Repository layer
- Create an internal interface that embeds a subset or all of the controller-runtime interfaces

This decision will influence how we encapsulate data access logic aka kube API access and how flexible our system will be for testing.

## Decision

We will use the `client.Client` interface provided by `controller-runtime` directly in the Repository layer and avoid creating custom interfaces or client implementations. This approach minimizes unnecessary abstraction and leverages the mature, well-tested interfaces provided by the Kubernetes ecosystem.

Whenever possible, we will reference the most specific sub-interface from the `client.Client` composition (e.g., `Reader`, `Writer`) rather than the full `Client` interface. This promotes better adherence to the Interface Segregation Principle and allows for more precise dependency injection, leading to simpler and more focused unit tests. If methods from multiple sub-interfaces are needed, the `client.Client` is used.

The client will **only** be referenced in the Repository layer. All other layers, such as Service and Reconciler, will remain decoupled from infrastructure concerns and interact only through higher-level abstractions defined in the application domain.

### Do's

A Service defines its dependecy named *Repository. The Repository implementation then uses controller-runtime's Client interfaces directly.
```go 
type SomeService struct {
	manifestRepository ManifestRepository
}

type ManifestRepository interface {
	Get(name, namespace string) (*v1beta2.Manifest, error)
}
```
```go
type ManifestRepositoryImpl struct {
	readClient client.Reader
}
```
Another example, for more than only read:
```go
type ManifestRepository interface {
    Get(name, namespace string) (*v1beta2.Manifest, error)
    Create(name, namespace string) error
    Update(*v1beta2.Manifest) error
}
```
```go
type ManifestRepositoryImpl struct {
	kcpClient client.Client
}
```

### Don'ts

The Service defines a dependency named *Client and uses it:
```go
type SomeService struct {
	manifestClient ManifestClient
}

type ManifestClient interface {
	Get(name, namespace string) (*v1beta2.Manifest, error)
}
```
Mitigation: It should be called *Repository.


The Service consumes the defined ManifestRespository interface as an embedded field:
```go
type SomeService struct {
	ManifestRepository
}

type ManifestRepository interface {
	Get(name, namespace string) (*v1beta2.Manifest, error)
}
```

Mitigation: It should be referenced as a named, private field so it can not be accessed across the package.


The Repository implementation defines its own intermediate composition interface from controller-runtime interfaces:
```go
type ManifestRepository struct {
	manifestClient manifestClient
}

type manifestClient interface {
	client.Writer
	client.Reader
}
```

Mitigation: It should use the `client.Client` directly if more than one sub interface is needed.

## Consequences

- The Repository layer becomes the single point of interaction with the Kubernetes API, improving separation of concerns and making the system easier to reason about
- By using specific sub-interfaces (e.g., Reader, Writer) when possible, dependencies are minimized, and unit testing becomes simpler and more focused.
- Higher layers (Service, Reconciler) remain infrastructure-agnostic, promoting separation of concerns and better testability
- We rely directly on the controller-runtime client interfaces, which reduces boilerplate but ties our implementation closely to that library’s abstractions
