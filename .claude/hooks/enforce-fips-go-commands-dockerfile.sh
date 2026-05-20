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

violations=$(echo "$proposed" | grep -En '^RUN .*go[[:space:]]+(build|test|run|install)' | grep -v 'GOFIPS140=v1\.0\.0' || true)

if [[ -n "$violations" ]]; then
    echo "FIPS check failed: 'go' used without 'GOFIPS140=v1.0.0' in Dockerfile RUN instruction:" >&2
    echo "$violations" >&2
    echo "Add 'GOFIPS140=v1.0.0' as an environment variable to go build, test, run, and install commands." >&2
    exit 2
fi
