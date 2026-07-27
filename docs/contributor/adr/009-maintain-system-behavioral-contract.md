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

Introduce and maintain a behavioral contract for the entire system (Lifecycle-Manager + Runtime Watcher).
The contract is intended to be a first-class artifact of our architecture documentation: It is the authoritative source of truth for system behavior and takes precedence over the source-code implementation or any other external document.

Every behavior-affecting implementation decision should be captured by the contract. The implementation should not introduce observable behavior that is not captured by the contract.
The contract is a living document and should be updated as the system evolves.
The contract is driven by stakeholder requirements. Within those constraints, we are free to define the contract as we see fit, provided it satisfies all defined requirements.

What exactly is captured by the contract? For example, the answers to the following questions with respect to the `Lifecycle-Manager's` behavior:
- What does `Lifecycle-Manager` do if OCM artifact for a Kyma module doesn't have a `Default-Module-CR` layer?
- What does `Lifecycle-Manager` do if user changes the namespace of a `Default-Module-CR` in an SKR cluster?
See below for more examples of contract items.


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

### Why not ADRs?

Why don't we just use ADRs to capture the behavioral contract?

An average ADR in this project, at the time of this writing, is 61 lines long.
Such an average ADR may result in one, two, or even five contract "items". Let's assume it will be two on average: An average behavior-related ADR will correspond to two contract "items".
Let's also assume that reconciliation of a `Default-Module-CR` requires 20 contract "items" to be well defined contract-wise. It will be 20 lines to read, if every item spans a single line.
If we use ADRs for that, it will (20/2)*61 = 610 lines to read. Twenty versus six hundred and ten lines is a huge difference.
ADRs are great for capturing decisions: They provide room for context, rationale, analysis of alternatives, and so on.
But they are bulky and verbose at the same time.
Most contract items should be backed up by existing ADRs but the ADRs themselves are not good tool for working on the contract - they are the tool for capturing decisions.

### Where to put the contract?

It should be a repository under our control. 
I don't see any problem with storing it in the `Lifecycle-Manager` repository.

Now, should the contract be stored in a single well-known location, or should it be defined at the source-code level, closer to the implementation?
Having the contract closer to the implementation looks like a good idea at first glance.
After all, the contract influences the implementation, so maybe these two things should be located close to each other?

Well, there is a problem with that approach. Once the contract is defined, implementation becomes a derivative of the contract.
More than that, the implementation may be changed (and will change) at any time. We're free to refactor, remove, split or merge packages and so on.
We may use AI Agents to very quickly refactor large parts of the project.
What happens to the contract documents stored in these directories being moved around? We'll have to constantly decide where to put them, and it will create a lot of friction.
In addition, the location of the contract documents will constantly change. This "uncertainity of location" is not providing any advantage in my opinion.
We should observe that the contract is superior to the implementation. If so, the contract's location shouldn't be affected by changes to the implementation that is derived from it.
Thus I conclude that the contract should be stored in a single well-known location that is not affected by changes in the source code structure.


## Decision

### Format
The contract is written in Markdown format. It has four sections:

- **Scope** - defines the scope of the contract. It describes what is covered by the document and what is not.
- **Decisions** - defines the contract items that are in effect
- **Pending Decisions** - defines the contract items that are not yet in effect. These may be contract gaps or things that are still under discussion, but are important enough to be visible.
- **Decisions References** - list of references to existing documents that provide context for the contract items. These may be ADRs, GitHub issues, end-user documentation, and so on.

The `Decisions` and `Pending Decisions` have a very simple table-based structure. Each contract item is a single row in the table. The columns of the table are:

- **ID** - A unique identifier of the contract item, useful when referencing the decision from other decisions or the source code. A `R010` is an example ID.
- **Description** - A short description of the contract item.
- **References** - A list of references to existing documents that provide context for the contract item. These may be ADRs, GitHub issues, end-user documentation, and so on. To keep the contract item minimalistic, the full reference link is provided in a separate section of the document. A `DL01` is an example document reference.
- **Comments** - Any additional comments that may be useful for understanding the contract item. Usually the comments should not be required as all the context should be provided by the references (ADRs, issues etc).

An example document is shown below (the actual content is not important, it is just an example of the structure):

```
## Scope

This document describes the contract for the handling the `Default-Module-CR` and `Module-CRD`.

## Decisions

| ID | Description | References | Comments |
| ------- | ----------------| ------------------- | ----- |
| R010 | When CRP is `CreateAndDelete`, `LM` deletes `Default-Module-CR` (if exists) upon module deprovisioning | DL02 ||
| R020 | `Default-Module-CR` instance is not actively reconciled | DL02, DL03 | \[DL02\]: "KLM does only Create and Delete it, but NOT reconcile the ModuleCR" |
| R030 | `LM` pauses resource deletion during module deprovisioning until all `Module-CRs` are deleted in the SKR | DL02 ||
| R040 | `Default-Module-CR` instance is created in the SKR **only** if the Module's `CustomResourcePolicy` is `CreateAndDelete` | DL03 ||
| R050 | `Default-Module-CR` instance is created in the SKR only once | DL03 ||
| R060 | `Default-Module-CR` instance is deleted in the SKR **only** if the Module's `CustomResourcePolicy` is `CreateAndDelete` | DL03 ||


## Pending decisions

| ID | Description | References | Comments |
| ------- | ----------------| ------------------- | ----- |
| PD005 | `Default-Module-CR` instance may be cluster-scoped | GAP | No ADR/Issue provides justification of this? |
| PD010 | For non-mandatory modules `LM` rejects module version without `Default-Module-CR` definition | DL01 | The rule is redundant by design, submission pipeline should guarantee that. |
| PD020 | `LM` accepts modules without `Module-CRD` defined in module's resources || We can treat this as an error and put the Manifest into error state, but perhaps Module Team has some other means to install the `Module-CRD` in the SKR? |
| PD030 | `LM` Ignores `Default-Module-CR` for Mandatory Modules | DL01 | According to DL01, Mandatory Modules should not have `Default-Module-CR`. Ignoring this misconfiguration seems to be best option |
| PD040 | When module is removed from Kyma module's list and no `Default-Module-CR` is found in the SKR, `LM` just waits until all `Module-CRs` are deleted|||
| PD050 | `LM` identifies `Default-Module-CR` by inspecting OCM Artifact corresponding to the module's version deployed in the SKR || Module Team may change name/namespace of the module between versions. KLM still should be able to find the `Default-Module-CR` for the currently deployed version |


### Decisions references
DL01: https://github.com/kyma-project/community/issues/982
DLO2: https://github.com/kyma-project/community/issues/972
DL03: https://github.com/kyma-project/lifecycle-manager/issues/3007

```

### Location

The contract documents are stored in a `Lifecycle-Manager` repository under the `docs/internal/contract` directory.
It is split into multiple documents, each corresponding to a specific scope of the contract.
The name of each document should correspond to it's scope. For example, the contract for the `Default-Module-CR` and `Module-CRD` is stored in the `docs/internal/contract/module-cr.md` file.


## Consequence

The contract is a living document. It should be updated as the system evolves.
It means that before implementing any behavior-affecting change to the system, we should first update the contract accordingly.
We should also spend some time to make sure that the contract is complete and there are no gaps in the design of our system.
We have to review other existing documents (like user-facing documentation) to make sure that they are consistent with the contract, and update these external documents if necessary.

