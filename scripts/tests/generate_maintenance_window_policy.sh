#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

OUTPUT_FILE="${1:?Usage: generate_maintenance_window_policy.sh <output-file-path>}"
mkdir -p "$(dirname "$OUTPUT_FILE")"

read -r NOW PLUS_2H PLUS_1D PLUS_1D_2H < <(python3 -c '
from datetime import datetime, timedelta, timezone
now = datetime.now(timezone.utc)
def t(delta): return (now + delta).strftime("%Y-%m-%dT%H:%M:%SZ")
print(t(timedelta()), t(timedelta(hours=2)), t(timedelta(days=1)), t(timedelta(days=1, hours=2)))
')

cat <<POLICY > "$OUTPUT_FILE"
{
  "rules": [
    {
      "match": {
        "region": "asia"
      },
      "windows": [
        {
          "begin": "$NOW",
          "end": "$PLUS_2H"
        }
      ]
    },
    {
      "match": {
        "region": "europe"
      },
      "windows": [
        {
          "begin": "$PLUS_1D",
          "end": "$PLUS_1D_2H"
        }
      ]
    }
  ],
  "default": {}
}
POLICY

echo "Maintenance window policy data written to: $OUTPUT_FILE"
echo "Maintenance window policy:"
cat "$OUTPUT_FILE"
