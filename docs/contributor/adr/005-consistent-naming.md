# ADR 005 - Consistent Naming

## Status

Accepted

## Context

It has to be decided how to consistently name things.

## Decision

It is decided that the following naming patterns apply:

### Layered Architecture

Major building blocks of the [layered architecture](004-layered-architecture.md) are *controllers*, *services* and *repositories*. It is decided that the types are suffixed accordingly. It is also explicitly decided to **NOT** use *Interface*  and *Impl*  suffixes.

#### Do's

```go
// package internal/controller/something
type SomeController struct { }
type SomeService interface { }

// package internal/service/something
type SomeService struct { }
type SomeRepository interface { }

// package internal/repository/something
type SomeRepository struct { }
```

## Consequence

We apply consistent naming within the repository.
This ADR may be extended with further naming guidelines.
