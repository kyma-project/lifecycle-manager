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

# Ensure recipe lines use $(GO) instead of bare 'go', so GOFIPS140=$(FIPS140_MODULE_VERSION) is always applied
violations=$(echo "$proposed" | grep -En $'^\t.*go[[:space:]]+(build|test|run|install)' || true)

if [[ -n "$violations" ]]; then
    echo "FIPS check failed: bare 'go' used instead of '\$(GO)' for compilation commands:" >&2
    echo "$violations" >&2
    echo "Use '\$(GO)' (which sets GOFIPS140=v1.0.0) for build, test, run, and install." >&2
    exit 2
fi

# If the proposed content redefines the GO variable, ensure GOFIPS140 is preserved
go_def=$(echo "$proposed" | grep -E '^GO[[:space:]]*[:?]?=' || true)

if [[ -n "$go_def" ]]; then
    if ! echo "$go_def" | grep -qF 'GOFIPS140=$(FIPS140_MODULE_VERSION)'; then
        echo "FIPS check failed: GO variable redefined without GOFIPS140=\$(FIPS140_MODULE_VERSION):" >&2
        echo "$go_def" >&2
        echo "The GO variable must be defined as: GO := GOFIPS140=\$(FIPS140_MODULE_VERSION) go" >&2
        exit 2
    fi
fi

# If the proposed content redefines FIPS140_MODULE_VERSION, ensure it stays at v1.0.0
fips_ver_def=$(echo "$proposed" | grep -E '^FIPS140_MODULE_VERSION[[:space:]]*[:?]?=' || true)

if [[ -n "$fips_ver_def" ]]; then
    if ! echo "$fips_ver_def" | grep -qF 'FIPS140_MODULE_VERSION := v1.0.0'; then
        echo "FIPS check failed: FIPS140_MODULE_VERSION must not be changed:" >&2
        echo "$fips_ver_def" >&2
        echo "The variable must be defined as: FIPS140_MODULE_VERSION := v1.0.0" >&2
        exit 2
    fi
fi
