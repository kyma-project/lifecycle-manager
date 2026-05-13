#!/usr/bin/env bash
set -euo pipefail

input=$(cat)

# For Edit, check new_string; for Write, check content
proposed=$(python3 -c "
import sys, json
d = json.load(sys.stdin)
ti = d.get('tool_input', {})
print(ti.get('new_string') or ti.get('content') or '')
" <<< "$input" 2>/dev/null || echo "")

if [[ -z "$proposed" ]]; then
    exit 0
fi

violations=$(echo "$proposed" | grep -En $'^\t.*go[[:space:]]+(build|test|run|install)' || true)

if [[ -n "$violations" ]]; then
    echo "FIPS check failed: bare 'go' used instead of '\$(GO)' for compilation commands:" >&2
    echo "$violations" >&2
    echo "Use '\$(GO)' (which sets GOFIPS140=v1.0.0) for build, test, run, and install." >&2
    exit 2
fi
