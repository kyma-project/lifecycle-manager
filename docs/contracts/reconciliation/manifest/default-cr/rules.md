### Rules

| Rule ID | Rule Description | Decision references | Labels |
| ------- | ----------------| ------------------- | ----- |
| R01 | All non-mandatory modules must define a Default-CR instance | DL01 ||
| R02 | Mandatory modules must NOT define a Default-CR instance | DL01 ||
| R03 | Default-CR instance is not actively reconciled |||
| R04 | Lifecycle-Manager pauses Module deletion until all CRs of Module-CRD type are deleted |||


### Decision references
DL01: https://github.com/kyma-project/community/issues/982

