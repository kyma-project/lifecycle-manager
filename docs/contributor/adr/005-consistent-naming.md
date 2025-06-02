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
// package internal/controller/foo
type FooController struct { }
type BarService interface { }

// package internal/service/bar
type BarService struct { }
type BazRepository interface { }

// package internal/repository/baz
type BazRepository struct { }
```

## Consequence

We apply consistent naming within the project.
This ADR may be extended with further naming guidelines.
