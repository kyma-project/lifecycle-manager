#!/usr/bin/env bash
set -o nounset
set -o errexit
set -E
set -o pipefail

OUTPUT_FILE="${1:?Usage: generate_maintenance_window_policy.sh <output-file-path>}"
mkdir -p "$(dirname "$OUTPUT_FILE")"

if [[ "$(uname)" == "Darwin" ]]; then
  current_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  time_plus_2_hours=$(date -u -v+2H +"%Y-%m-%dT%H:%M:%SZ")
  time_plus_1_day=$(date -u -v+1d +"%Y-%m-%dT%H:%M:%SZ")
  time_plus_1_day_plus_2_hours=$(date -u -v+1d -v+2H +"%Y-%m-%dT%H:%M:%SZ")
else
  current_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  time_plus_2_hours=$(date -u -d "2 hours" +"%Y-%m-%dT%H:%M:%SZ")
  time_plus_1_day=$(date -u -d "1 day" +"%Y-%m-%dT%H:%M:%SZ")
  time_plus_1_day_plus_2_hours=$(date -u -d "1 day 2 hours" +"%Y-%m-%dT%H:%M:%SZ")
fi

cat <<POLICY > "$OUTPUT_FILE"
{
  "rules": [
    {
      "match": {
        "region": "asia"
      },
      "windows": [
        {
          "begin": "$current_time",
          "end": "$time_plus_2_hours"
        }
      ]
    },
    {
      "match": {
        "region": "europe"
      },
      "windows": [
        {
          "begin": "$time_plus_1_day",
          "end": "$time_plus_1_day_plus_2_hours"
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
