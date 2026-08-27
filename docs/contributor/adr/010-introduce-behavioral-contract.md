# ADR 009 - Maintain System Behavioral Contract

## Status

Proposed


## Context

### Problem Statement

We're building a system that is based on a set of requirements. These are currently defined in many places: ADRs, end-user documentation, GitHub issues in several repositories, and so on.
Some of these requirements are just embedded in the code without any proper documentation.
Because of the above, we're in a grim situation: we don't know exactly how many rules we have implemented and how many gaps exist in the design of our system.
We don't know if some behaviors are actually required by stakeholders or if they are just side effects of the implementation.
And every time we want to find out why certain behavior was implemented in a certain way, we have to again read through ADRs, decision logs, GitHub issues, end-user documentation... Hoping to find an answer. This is a very time-consuming and error-prone process.
This ADR is an attempt to address this situation by introducing a systematic way to define and maintain a behavioral contract for our system.

### Solution

Introduce and maintain a behavioral contract. To limit the scope, this ADR only addresses the contract for the `Module-CR` handling.
If successful, the same approach can be applied to other areas of the system.

The contract is intended to be a first-class artifact of our architecture documentation: It is the authoritative source of truth for system behavior and takes precedence over the source-code implementation.

Every important behavior-affecting implementation decision should be captured by the contract. The implementation should not introduce observable behavior that is not captured by the contract.
The contract is a living document and should be updated as the system evolves.
The contract is driven by stakeholder requirements. Within those constraints, we are free to define the contract as we see fit, provided it satisfies all defined requirements.

What exactly is captured by the contract? For example, the answers to the following questions with respect to the `Lifecycle-Manager's` behavior:
- What does `Lifecycle-Manager` do if OCM artifact for a Kyma module doesn't have a `Default-Module-CR` layer?
- What does `Lifecycle-Manager` do if user changes the namespace of a `Default-Module-CR` in an SKR cluster?
and so on.
See below for an example of a contract document.


### Contract Characteristics

The contract should have the following characteristics:

- **Internal** - The contract is an internal artifact of our team. It is not intended for end users, it is not a public API. It is a tool for us to drive the system's design and implementation.
- **Stakeholder-driven** - The contract is driven by stakeholder requirements. It should capture all behavior-affecting implementation aspects that are required by stakeholders. Nevertheless, we are free to define the contract as we see fit, provided it satisfies all stakeholder requirements.
- **Authoritative** - The contract is the authoritative source of truth for system behavior. It takes precedence over the source-code implementation.
- **Under our control** - The contract is under our control. We can define it as we consider appropriate, provided it satisfies all stakeholder requirements. We don't need to get approval from stakeholders or TWs for the contents, format or the location of the contract.
- **Living document** - The contract is a living document. It should be updated as the system evolves. It should be versioned and changes should be tracked.
- **Gaps are documented** - The contract should, at least initially, allow to document gaps in the design of our system. With that we can better understand the current state of our architecture and identify areas that require improvement.
- **Easy to read and maintain** - The contract should be easy to read and understand. Because of that, it should be minimalistic and concise.
- **Backed up by exiting documents** - Contract items, if directly related to internal or external decisions, should be backed up by existing documents: ADRs, links to GitHub issues, and so on. This allows to trace the important context for every requirement, if necessary. Such context should never be added to the contract item itself, because it will make the contract bulky and verbose.
- **Convenient to use by an AI Agent** - The contract should be convenient to use by an AI Agent. It should be structured in a way that makes it "cheap" for an AI Agent to access and process it.
- **Pragmatic** - The goal is not to introduce a "Markdown-driven" development process. Initially, the contract should only focus on capturing the most important, high-level behaviors and identifying gaps within these. Once it is done we'll decide on how fine-grained the contract should be.

### Why not ADRs?

Why don't we just use ADRs to capture the behavioral contract?

An average ADR in this project, at the time of this writing, is 61 lines long.
Such an average ADR may result in one, two, or even five contract "items". Let's assume it will be two on average: An average behavior-related ADR corresponds to two contract "items".
Let's also assume that reconciliation of a `Default-Module-CR` requires 20 contract "items" to be well defined contract-wise. It will be 20 lines to read, if every item spans a single line.
If we use ADRs for that, it will (20/2)*61 = 610 lines to read. Twenty versus six hundred and ten lines is a huge difference.
ADRs are great for capturing decisions: They provide room for context, rationale, analysis of alternatives, and so on.
But they are bulky and verbose at the same time.
Most contract items should be backed up by existing ADRs but the ADRs themselves are not good tool for working on the contract - they are the tool for capturing decisions.

## Decision

### Scope

The scope of this ADR is limited to the contract for the `Module-CR` handling. This contract is scoped to the - currently non-existent - "module-cr-service" that will be responsible for implementing it.


### Location

By Team's agreement, the contract documents will be stored in the service layer directories, close to the services they describe.
For example, the contract for the `Module-CR` handling should be stored in the `Lifecycle-Manager` repository under the `internal/service/modulecr` directory (or similar).


### Format

By Team's agreement, the contract document format is not specified - on purpose. It's format should be decided on a case-by-case basis.
Once few contracts are defined, we can decide on a common format for all contracts, if necessary.
The only specific requirement is that the contract for `Module-CR` handling should initially also describe **gaps** in the current implementation / architecture.
The gaps are rules (contract items) that are required by stakeholders or logically implied by other rules, but are not yet properly implemented, including cases for which the current implementation yields and unspecified behavior.
Having all gaps documented close to the contract will help us to better understand the overall "health" of the contract.
It is expected that over time all the gaps will be resolved and so the final contract will not contain any.


## Consequence

The contract is a living document. It should be updated as the system evolves.
It means that before implementing any behavior-affecting change to the system, we should first update the contract accordingly.
We should work iteratively over the contract to make sure that it is complete and there are no gaps in the design of our system.
We also have to review other existing documents (like user-facing documentation) to make sure that they are consistent with the contract, and update these external documents if necessary.
To keep things practical and avoid excessive formalism a pragmatic approach is required: Initially we should focus only on the most important, high level behaviors, gradually documenting lower-level aspects if necessary. Deciding which behaviors are important enough to be captured by the contract and which should be left to the implementation is a matter of pragmatism and good judgment and it will come with time.
