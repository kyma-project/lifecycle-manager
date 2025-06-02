# ADR 005 - Consistent Naming

## Status

Accepted

## Context

It has to be decided how to consistently name things.

## Decision

It is decided that the following naming patterns apply:

### Layered Architecture

Major building blocks of the [layered architecture](004-layered-architecture.md) are *controllers*, *services* and *repositories*. It is decided that the types are suffixed accordingly. It is also explicitly decided to **NOT** use *Interface*  and *Impl*  suffixes. Further, the implementation types are not prefixed with the "context" as this is established by the package already.

#### Do's

```go
// package internal/controller/foo
type Controller struct { } // => foo.Controller
type BarService interface { }

// package internal/service/bar
type Service struct { } // => bar.Service
type BazRepository interface { }

// package internal/repository/baz
type Repository struct { } // => bar.Repository
```

## Consequence

We apply consistent naming within the project.
This ADR may be extended with further naming guidelines.
