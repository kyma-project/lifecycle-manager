#!/bin/bash
input=$(cat)
file_path=$(echo "$input" | jq -r '.tool_input.file_path // empty')

[[ "$file_path" == *.go ]] || exit 0

PROJECT_ROOT="$PWD"
dir=$(dirname "$file_path")

lint_fix_and_report() {
  local target=$1
  local pkg=$2
  local output
  output=$(make -C "$PROJECT_ROOT" "$target" PKG="$pkg" 2>&1)
  local exit_code=$?
  if [[ $exit_code -ne 0 ]]; then
    echo "$output" | jq -Rs \
      '{hookSpecificOutput: {hookEventName: "PostToolUse", additionalContext: ("Remaining lint issues that require manual fixes:\n" + .)}}'
    exit 1
  fi
}

if [[ "$file_path" == "${PROJECT_ROOT}/api/"* ]]; then
  rel_dir="${dir#${PROJECT_ROOT}/api/}"
  [[ "$rel_dir" == "$dir" ]] && rel_dir="."
  lint_fix_and_report "lint-fix-api" "./${rel_dir}/..."
elif [[ "$file_path" == "${PROJECT_ROOT}/maintenancewindows/"* ]]; then
  rel_dir="${dir#${PROJECT_ROOT}/maintenancewindows/}"
  [[ "$rel_dir" == "$dir" ]] && rel_dir="."
  lint_fix_and_report "lint-fix-maintenancewindows" "./${rel_dir}/..."
else
  rel_dir="${dir#${PROJECT_ROOT}/}"
  [[ "$rel_dir" == "$dir" ]] && rel_dir="."
  lint_fix_and_report "lint-fix-pkg" "./${rel_dir}/..."
fi
