### Rules


| Rule ID | Rule Description | Decision references | Comments |
| ------- | ----------------| ------------------- | ----- |
| R010 | `LM` rejects module version without `Default-Module-CR` definition | DL01 ||
| R020 | `Default-Module-CR` instance is not actively reconciled | DL02, DL03 | \[DL02\]: "KLM does only Create and Delete it, but NOT reconcile the ModuleCR" |
| R030 | `LM` pauses resource deletion during module deprovisioning until all `Module-CRs` are deleted in the SKR | DL02 ||
| R040 | `Default-Module-CR` instance is created in the SKR **only** if the Module's `CustomResourcePolicy` is `CreateAndDelete` | DL03 ||
| R050 | `Default-Module-CR` instance is created in the SKR only once | DL03 ||
| R060 | `Default-Module-CR` instance is deleted in the SKR **only** if the Module's `CustomResourcePolicy` is `CreateAndDelete` | DL03 ||
| R070 | `Default-Module-CR` is deleted in the SKR, if exists, to notify Module's operator about the Module deprovisioning | DL03 ||


### Pending decisions
| Rule ID | Rule Description | Decision references | Comments |
| ------- | ----------------| ------------------- | ----- |
| PD01 | `LM` accepts modules without `Module-CRD` defined in module's resources || We can treat this as an error and put the Manifest into error state, but perhaps Module Team has some other means to install the `Module-CRD` in the SKR? |
| PD02 | `LM` Ignores `Default-Module-CR` for Mandatory Modules | DL01 | According to DL01, Mandatory Modules should not have `Default-Module-CR`. Ignoring this misconfiguration seems to be best option |
| PD03 | When module is removed from Kyma module's list and no `Default-Module-CR` is found in the SKR, `LM` just waits until all `Module-CRs` are deleted|||
| PD04 | `LM` identifies `Default-Module-CR` by inspecting OCM Artifact corresponding to the module's version deployed in the SKR || Module Team may change name/namespace of the module between versions. KLM still should be able to find the `Default-Module-CR` for the currently deployed version |


### Decisions references
DL01: https://github.com/kyma-project/community/issues/982
DLO2: https://github.com/kyma-project/community/issues/972
DL03: https://github.com/kyma-project/lifecycle-manager/issues/3007
