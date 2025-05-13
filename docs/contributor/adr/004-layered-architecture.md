# ADR 004 - Adoption of Layered Architecture (3-Tier)

## Status

Accepted

## Context

We need to design a maintainable, testable, and scalable architecture for lifecycle-manager, which interacts with Kubernetes APIs and includes business logic and orchestration of resources.

## Decision

To achieve this, we have adopted a Layered Architecture pattern with the following three layers:
1. Controller Layer (Top)
2. Service Layer (Middle)
3. Repository Layer (Bottom) 

Each layer has distinct responsibilities and clear dependency rules to ensure separation of concerns and minimize coupling.

### Controller Layer
Responsibility:
- Serves as the orchestrator for reconciling Kubernetes resources
- Handles event processing and decision-making around requeue intervals and error handling

Key Points:
- Depends on Service Layer
- Uses the results from service calls to make decisions about reconciliation behavior
- It is the only layer that creates and manages controller-runtime’s ctrl.Result objects
- Does not perform business logic directly; delegates all such work to services

### Service Layer
Responsibility:
- Implements business logic by orchestrating one or more repositories
- Encapsulates complex workflows and business rules
- Mayay also depend on or reference other services to reuse common business logic

Key Points:
- Depends on Repository Layer
- Must not access Kubernetes APIs directly; instead, uses repositories for any data access
- Contains pure business logic to ensure it can be tested independently of Kubernetes

### Repository Layer
Responsibility:
- Acts as a pure CRUD (Create, Read, Update, Delete) data layer
- Directly interacts with the Kubernetes API using the controller-runtime client
- Responsible for all data persistence and retrieval operations
- Does not contain any business logic

Key Points:
- This is the only layer that has direct access to the Kubernetes API
- Provides a clean abstraction over client calls to isolate API details from the rest of the application

#### Dependency Direction
![dep dir](./../assets/layered-arch-dep-dir.svg)

- **No layer may reference or depend on a higher layer**
- This strict direction ensures that dependencies flow **only downward**, reducing tight coupling and enforcing separation of concerns

#### Advantages

- Separation of Responsibilities:
    - Each layer has a single, well-defined purpose, improving maintainability and readability
- Testability:
    - Services can be tested independently of Kubernetes by mocking repositories
    - Controllers can be tested by mocking services
- Encapsulation:
    - The repository abstracts the Kubernetes API, allowing changes in API details with minimal impact
    - Business logic is confined to the Service layer, so no other layer will contain side-effects to the business code
- Flexibility & Scalability:
    - Business logic is decoupled from infrastructure details, allowing for easier scaling and adaptation of business requirements
- Clear Ownership:
    - Only the controller layer handles ctrl.Result, ensuring a clear and centralized orchestration of the reconciliation loop
- Ease of Navigation:
    - The agreed-upon architecture provides a predictable structure, making it easy for developers to understand, navigate, and contribute to the codebase efficiently

## Consequences
- Developers must respect the dependency rules to avoid circular dependencies or logic leakage across layers.
- Layer violations can lead to tight coupling and brittle code.
- Extra boilerplate may be needed to maintain strict separation.
