### Rules

| Rule ID | Rule Description | Decision references | Labels |
| ------- | ----------------| ------------------- | ----- |
| R01 | All non-mandatory modules must define a Module-CR instance | DL01 ||
| R02 | Mandatory modules must NOT define a Module-CR instance | DL01 ||
| R03 | Module-CR instance is not actively reconciled | DL02 ||
| R04 | Lifecycle-Manager pauses Module deletion until all `Module-CRs` are deleted | DL02 ||


### Decisions references
DL01: https://github.com/kyma-project/community/issues/982
DLO2: https://github.com/kyma-project/community/issues/972

