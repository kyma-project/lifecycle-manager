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
