#!/usr/bin/env bash
#
# compare.sh - the parity gate. Normalize a golden render and a candidate
# render, then diff them object-by-object. Exit non-zero on ANY semantic
# difference. This is the check every migration phase must pass.
#
# Usage:
#   compare.sh <golden.yaml> <candidate.yaml> [label]
#
# Exit codes:
#   0  renders are semantically identical
#   1  a difference was found (details printed)
#   2  usage error
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NORMALIZE="${SCRIPT_DIR}/normalize.sh"

usage() { echo "usage: $(basename "$0") <golden.yaml> <candidate.yaml> [label]" >&2; exit 2; }

[[ $# -ge 2 ]] || usage
GOLDEN="$1"
CANDIDATE="$2"
LABEL="${3:-parity}"

[[ -f "$GOLDEN" ]]    || { echo "error: golden not found: $GOLDEN" >&2; exit 1; }
[[ -f "$CANDIDATE" ]] || { echo "error: candidate not found: $CANDIDATE" >&2; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
G="${WORK}/golden"
C="${WORK}/candidate"

"$NORMALIZE" "$GOLDEN"    "$G" >&2
"$NORMALIZE" "$CANDIDATE" "$C" >&2

# Union of object identities across both sides. Written to a temp file and read
# with a portable `while read` loop so the harness works on bash 3.2 (macOS
# default), which lacks `mapfile`.
KEYS_FILE="${WORK}/keys.txt"
( cd "$WORK" && ls golden candidate 2>/dev/null | grep '\.json$' | sort -u ) > "$KEYS_FILE"

rc=0
n_ok=0; n_diff=0; n_only_g=0; n_only_c=0
echo "=== parity report: ${LABEL} ==="
while IFS= read -r k; do
  [[ -n "$k" ]] || continue
  gf="${G}/${k}"; cf="${C}/${k}"
  obj="${k%.json}"
  if [[ -f "$gf" && ! -f "$cf" ]]; then
    echo "  MISSING (in golden, absent in candidate): ${obj}"
    n_only_g=$((n_only_g + 1)); rc=1
  elif [[ ! -f "$gf" && -f "$cf" ]]; then
    echo "  EXTRA   (in candidate, absent in golden): ${obj}"
    n_only_c=$((n_only_c + 1)); rc=1
  elif ! diff -q "$gf" "$cf" >/dev/null; then
    echo "  DIFF    ${obj}"
    diff -u "$gf" "$cf" | sed 's/^/      /' || true
    n_diff=$((n_diff + 1)); rc=1
  else
    n_ok=$((n_ok + 1))
  fi
done < "$KEYS_FILE"

echo "--- summary: ${n_ok} identical, ${n_diff} changed, ${n_only_g} golden-only, ${n_only_c} candidate-only ---"
if [[ $rc -eq 0 ]]; then
  echo "PASS: ${LABEL} renders are semantically identical"
else
  echo "FAIL: ${LABEL} renders differ"
fi
exit $rc
