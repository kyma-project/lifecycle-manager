## Goal and Scope

The goal of this document is to define the rules for handling the `Default-Module-CR` and `Module-CRD`.
The idea is to have a list of simple rules that are enough to describe how the system should behave in all situations, without leaving any gaps.
Every rule should be backed by a decision reference, which is a link to an ADR, an issue, or an existing document that provides justification for the rule, and maybe also a view on possible alternatives.

Rules resulting in non-obvious outcome may be further explained by provindg runtime scenarios (see scenarios.md), but it's not required.

This document is intended to be **minimalistic** and **complete**. It should not contain any implementation details, but only describe the expected behavior of the system. 

What is the benefit of having such a document?
Currently, in order to understand the behavior of the system, one has to read through multiple ADRs, issues, and documents.
The problem is, there is no compiled list of: "documents that you have to read in order to understand `Default-Module-CR reconciliation`".
**This** document is intended to provide such a list.
In addition, this document presents **refined**, **distilled**, **minimalistic** set of rules, that - in theory - should be enough to re-implement the `Default-Module-CR` reconciliation logic from scratch.
It should be a first-class reference for both human readers and AI Agents.
It should also serve as a "source-of-truth" for implementation: Source code is just an artifact derived from an existing, and well-described contract, NOT the other way around.


### Rules

| Rule ID | Rule Description | Decision references | Comments |
| ------- | ----------------| ------------------- | ----- |
| R010 | `LM` deletes `Default-Module-CR` (if exists) upon module deprovisioning | DL02 | "lifecycle-manager will continue to mark the default Module CR for deletion (not any other customer-created Module CR)" |
| R020 | `Default-Module-CR` instance is not actively reconciled | DL02, DL03 | \[DL02\]: "KLM does only Create and Delete it, but NOT reconcile the ModuleCR" |
| R030 | `LM` pauses resource deletion during module deprovisioning until all `Module-CRs` are deleted in the SKR | DL02 ||
| R040 | `Default-Module-CR` instance is created in the SKR **only** if the Module's `CustomResourcePolicy` is `CreateAndDelete` | DL03 ||
| R050 | `Default-Module-CR` instance is created in the SKR only once | DL03 ||
| R060 | `Default-Module-CR` instance is deleted in the SKR **only** if the Module's `CustomResourcePolicy` is `CreateAndDelete` | DL03 ||



### Pending decisions

| Rule ID | Rule Description | Decision references | Comments |
| ------- | ----------------| ------------------- | ----- |
| PD005 | `Default-Module-CR` instance may be cluster-scoped | GAP | No ADR/Issue provides justification of this? |
| PD010 | `LM` rejects module version without `Default-Module-CR` definition | DL01 | The rule is redundant by design, submission pipeline should guarantee that. Nevertheless, our contract should be COMPLETE (leave no obivious gaps). |
| PD020 | `LM` accepts modules without `Module-CRD` defined in module's resources || We can treat this as an error and put the Manifest into error state, but perhaps Module Team has some other means to install the `Module-CRD` in the SKR? |
| PD030 | `LM` Ignores `Default-Module-CR` for Mandatory Modules | DL01 | According to DL01, Mandatory Modules should not have `Default-Module-CR`. Ignoring this misconfiguration seems to be best option |
| PD040 | When module is removed from Kyma module's list and no `Default-Module-CR` is found in the SKR, `LM` just waits until all `Module-CRs` are deleted|||
| PD050 | `LM` identifies `Default-Module-CR` by inspecting OCM Artifact corresponding to the module's version deployed in the SKR || Module Team may change name/namespace of the module between versions. KLM still should be able to find the `Default-Module-CR` for the currently deployed version |


### Decisions references
DL01: https://github.com/kyma-project/community/issues/982
DLO2: https://github.com/kyma-project/community/issues/972
DL03: https://github.com/kyma-project/lifecycle-manager/issues/3007
