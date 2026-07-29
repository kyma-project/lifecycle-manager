# Chart parity gate

This directory holds the **golden-render parity harness** that guards the
Helm-chart migration. Its job is to prove that a candidate `lifecycle-manager`
chart renders **semantically identically** to the current production chart in
[`management-plane-charts`](https://github.com/kyma-project/management-plane-charts),
for both certificate backends.

Production render must not change during the migration. This gate is the
evidence.

## Files

| File | Purpose |
|---|---|
| `render.sh` | Renders a chart with pinned production-like inputs for one cert backend (`--cm` or `--gcm`). Takes the chart path as an argument, so it renders both the production chart and, later, the in-repo chart. |
| `normalize.sh` | Explodes a multi-document render into one file per object and deep-sorts each object's map keys. Neutralizes document order and key order only. |
| `compare.sh` | The gate. Normalizes a golden and a candidate render, diffs object-by-object, exits non-zero on any difference. |
| `capture-golden.sh` | Regenerates the committed golden baseline from the production chart at the pinned ref. |
| `values-prod.yaml` | Pinned production-like render inputs (the three `required`-gated landscape keys). |
| `golden/prod-cm.yaml` | Frozen production render, cert-manager backend. |
| `golden/prod-gcm.yaml` | Frozen production render, gardener backend. |

## Running the gate

Compare a candidate chart against the committed golden, both backends:

```bash
cd scripts/chart-parity
./render.sh  --cm  <candidate-chart-path> > /tmp/cand-cm.yaml
./compare.sh golden/prod-cm.yaml  /tmp/cand-cm.yaml  "candidate CM"

./render.sh  --gcm <candidate-chart-path> > /tmp/cand-gcm.yaml
./compare.sh golden/prod-gcm.yaml /tmp/cand-gcm.yaml "candidate GCM"
```

`compare.sh` exits `0` when renders are identical and `1` on any difference,
printing a per-object `diff` for every object that changed, was added, or was
removed.

## What "semantically identical" means

The normalizer neutralizes **only cosmetic noise**:

- **Document order** — objects are keyed by identity
  (`apiVersion__kind__namespace__name`), so stream order cannot cause a false
  diff.
- **Map key order** — each object is emitted as JSON with keys deep-sorted, so
  key ordering cannot cause a false diff. This is the CRD ordering noise noted
  in the migration plan.

Everything else stays sensitive. Array element order (RBAC `.rules`, container
args), every value, label, and annotation are compared exactly. A reordered
rules list or a changed flag value is a real render change and fails the gate.

## Regenerating the golden baseline

The golden files are the frozen production render, committed so the gate runs
without a `management-plane-charts` checkout.

They are captured from the production chart at pinned ref **`cf59114e8`**
("KLM Release 1.20.4"). At or past release 1.20.4 the chart's manager Role
already matches `config/rbac`, so the RBAC drift described in the migration plan
is not present in the baseline. Capturing against an older checkout would
re-introduce that resolved drift, so `capture-golden.sh` refuses to run unless
the source repo is at the pinned ref.

```bash
MP_CHARTS_CHART=/path/to/management-plane-charts/lifecycle-manager \
  ./capture-golden.sh
```

To intentionally rebaseline against a newer production release, update
`PINNED_REF` in `capture-golden.sh` and the ref in this README, then rerun.

## Requirements

`helm`, `yq`, `jq`, and `bash`. The scripts are written for bash 3.2 (the macOS
system default) and work unchanged on newer bash in CI.
