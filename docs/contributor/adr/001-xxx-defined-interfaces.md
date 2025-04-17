# ADR 001 - XXX-Defined Interfaces

## Status

Accepted

## Context

In contrast to many other programming languages, Go supports the concept of consumer-defined interfaces that are fulfilled implicitly.

This pattern seems to be the recommended approach in Go, see for example:

- https://go.dev/wiki/CodeReviewComments#interfaces
- https://www.thoughtworks.com/en-de/insights/blog/programming-languages/mistakes-to-avoid-when-coming-from-an-object-oriented-language
- https://victorpierre.dev/blog/five-go-interfaces-best-practices/

Key arguments for consumer-defined interfaces are:

```diff
+ consumer may define the exact set of needed functionality (interface segregation)
+ consumer dependencies kept to a minimum (no import of the externally-defined interface)
+ producer may change without requiring code-level changes in the consumer (e.g., use adaptor to make it comply to the old interface)
- the same interface may be defined multiple times by different consumers
```

The question is, whether there are still scenarios in Go where producer-defined interfaces may be preferred over consumer-defined ones.

Key arguments for producer-defined interfaces are:

```diff
+ easier to trace where the producer is consumed and to find which consumers will break upon changes of the producer
```

It needs to be decided what criteria shall be used to choose between consumer-defined and provider-defined interfaces, or whether we want to unifromly follow one pattern where the other is only used in exceptional cases.

## Decision

TBD

## Consequence

TBD
