# ADR 005 - Consistent Naming

## Status

Accepted

## Context

We must decide how to consistently name things in the project.

## Decision

It is decided that the following naming patterns apply:

### Layered Architecture

Major building blocks, namely types, of the [layered architecture](004-layered-architecture.md) are *controllers*, *services*, and *repositories*. It is decided that:
- The types are suffixed accordingly.
- We **DON'T** use *Interface*  and *Impl*  suffixes. 
- The implementation types are not prefixed with the context. The context is already established by the package name.

#### Do's

```go
// package internal/controller/foo
type Controller struct { } // => foo.Controller
type BarService interface { }

// package internal/service/bar
type Service struct { } // => bar.Service
type BazRepository interface { }

// package internal/repository/baz
type Repository struct { } // => baz.Repository
```

## Consequences

We apply consistent naming within the project.
This ADR may be extended with further naming guidelines.
