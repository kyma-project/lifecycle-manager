#!/usr/bin/env bash
#
# normalize.sh - explode a multi-document Helm/Kustomize render into one file
# per object, neutralizing ONLY cosmetic noise so the parity diff stays honest.
#
# What is neutralized:
#   - document order   : each object is written to a file keyed by its identity
#                        (apiVersion__kind__namespace__name), so stream order
#                        cannot cause a false diff.
#   - object key order : each object is emitted as JSON with keys deep-sorted
#                        (jq -S), so map key ordering cannot cause a false diff
#                        (this is the CRD ordering noise called out in plan §2.4).
#
# What is deliberately NOT touched (kept maximally sensitive):
#   - array element order (e.g. RBAC .rules, container args) is preserved.
#     Two helm renders of near-identical charts must already agree on order; a
#     reordering is a real render change and should surface as a diff.
#   - no values, labels, or annotations are stripped.
#
# Usage:
#   normalize.sh <input.yaml> <output-dir>
set -euo pipefail

usage() { echo "usage: $(basename "$0") <input.yaml> <output-dir>" >&2; exit 2; }

[[ $# -eq 2 ]] || usage
INPUT="$1"
OUTDIR="$2"

[[ -f "$INPUT" ]] || { echo "error: input not found: $INPUT" >&2; exit 1; }

mkdir -p "$OUTDIR"
# Start clean so stale objects from a previous run cannot mask a removal.
rm -f "$OUTDIR"/*.json 2>/dev/null || true

# Collect all non-null documents into a single JSON array. `yq ea` (eval-all)
# reads the whole stream; select(. != null) drops the empty documents helm
# emits between templates.
ALL_JSON="$(yq ea -o=json '[select(. != null)]' "$INPUT")"

N="$(jq 'length' <<<"$ALL_JSON")"
if [[ "$N" -eq 0 ]]; then
  echo "warning: no objects found in $INPUT" >&2
fi

sanitize() { echo "$1" | tr '/:' '--' | tr -c 'A-Za-z0-9._-' '_'; }

for ((i = 0; i < N; i++)); do
  obj="$(jq ".[$i]" <<<"$ALL_JSON")"
  api="$(jq -r '.apiVersion // "NoApiVersion"' <<<"$obj")"
  kind="$(jq -r '.kind // "NoKind"' <<<"$obj")"
  ns="$(jq -r '.metadata.namespace // "_cluster"' <<<"$obj")"
  name="$(jq -r '.metadata.name // "NoName"' <<<"$obj")"

  key="$(sanitize "${api}__${kind}__${ns}__${name}")"
  target="${OUTDIR}/${key}.json"

  if [[ -e "$target" ]]; then
    echo "error: duplicate object identity: ${api} ${kind} ${ns}/${name}" >&2
    exit 1
  fi

  # jq -S sorts map keys recursively; array order is preserved.
  jq -S . <<<"$obj" > "$target"
done

echo "normalized $N object(s) into $OUTDIR" >&2
