# ADR 001 - XXX-Defined Interfaces

- status: work in progress
- date: 2025-04-08
- contributors:
  - @c-pius
  - @ruanxin

## Context

In contrast to many other programming languages, Go supports the concept of consumer-defined interfaces that are implemented implicitly.

Overall, this pattern seems to be the preferred approach in Go, see e.g.:

- https://go.dev/wiki/CodeReviewComments#interfaces
- https://www.thoughtworks.com/en-de/insights/blog/programming-languages/mistakes-to-avoid-when-coming-from-an-object-oriented-language
- https://victorpierre.dev/blog/five-go-interfaces-best-practices/

Still, there may be scenarios where producer-defined interfaces may be preferred over consumer-defined ones.

It needs to be decided what criteria shall be used to choose between consumer-defined and provider-defined interfaces, or whether we want to unifromly follow one pattern where the other is only used in exceptional cases.

## Decision

TBD

## Consequence

TBD

## Discussion

TBD
