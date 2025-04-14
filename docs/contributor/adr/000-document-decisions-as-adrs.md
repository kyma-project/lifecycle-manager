# ADR 000 - Decisions are documented as ADRs in `/docs/contributor/adr`

## Status

Accepted

## Context

Continuously working on lifecycle-manager continuously requires decisions to be taken concerning lifecycle-manager.

It needs to be decided where and how such decisions are documented.

## Decision

It is decided that outside-facing decisions that are aligned with other Kyma teams are continued to be documented as Issues in the `kyma-project/community` repository. This is the general approach other teams follow as well.

For decisions internal to lifecycle-manager, for example, implementation- or architecture-related decisions, the following applies:

- decisions MUST follow the [ADR template by Michael Nygard](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
- decisions MUST be stored as Markdown files in `/docs/contributor/adr`
- decisions MUST be aligned within the @kyma-project/jellyfish team
- decisions MUST be contributed as a PR

## Consequences

Each new significant decision about internal lifecycle-manager implementation or architecture is documented as an ADR.
